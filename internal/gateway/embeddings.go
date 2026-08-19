package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/provider/types"
)

// POST /v1/embeddings -- what every RAG pipeline calls before it calls
// anything else.
//
// It differs from the chat endpoints in one structural way: the caller
// names the model it wants and means it. Routing rules exist to pick a
// chat model on the caller's behalf by task condition; silently sending
// an embedding request to a different model than asked would return
// vectors from a different space, which is not a degraded answer but a
// wrong one. So this resolves the named model and refuses if it is not
// available, rather than routing.
//
// Everything else is the same pipeline: DLP over the input text, the
// key's model scope, the per-call ceiling, a quota reservation before
// the call and settlement after.

type embeddingsRequest struct {
	Model string          `json:"model"`
	Input json.RawMessage `json:"input"`

	raw map[string]json.RawMessage
}

func decodeEmbeddingsRequest(r io.Reader) (embeddingsRequest, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return embeddingsRequest{}, err
	}
	req := embeddingsRequest{raw: raw, Input: raw["input"]}
	if err := decodeField(raw, "model", &req.Model); err != nil {
		return embeddingsRequest{}, err
	}
	if len(req.Input) == 0 || string(req.Input) == "null" {
		return embeddingsRequest{}, fmt.Errorf("gateway: embeddings: input is required")
	}
	return req, nil
}

// texts pulls the scannable strings out of input, which OpenAI allows to
// be a string, an array of strings, or arrays of token ids. Token arrays
// carry no text for DLP to read and are passed through untouched.
func (req embeddingsRequest) texts() []string {
	if s, err := jsonString(req.Input); err == nil {
		return []string{s}
	}
	var many []string
	if err := json.Unmarshal(req.Input, &many); err == nil {
		return many
	}
	return nil
}

// withTexts rebuilds the body with the masked strings put back, leaving
// every other field the caller sent exactly as it was.
func (req embeddingsRequest) withTexts(texts []string, modelIdentifier string) ([]byte, error) {
	out := make(map[string]json.RawMessage, len(req.raw)+1)
	for k, v := range req.raw {
		out[k] = v
	}

	model, err := json.Marshal(modelIdentifier)
	if err != nil {
		return nil, err
	}
	out["model"] = model

	if len(texts) > 0 {
		var encoded []byte
		if _, err := jsonString(req.Input); err == nil {
			encoded, err = json.Marshal(texts[0])
			if err != nil {
				return nil, err
			}
		} else {
			encoded, err = json.Marshal(texts)
			if err != nil {
				return nil, err
			}
		}
		out["input"] = encoded
	}

	return json.Marshal(out)
}

func (p *Pipeline) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	key, ok := p.authenticate(w, r)
	if !ok {
		return
	}

	req, err := decodeEmbeddingsRequest(r.Body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, err.Error())
		return
	}

	// DLP reads the same content here as anywhere else: an embedding
	// request is text on its way to a third party like any other.
	texts := req.texts()
	messages := make([]chatMessage, len(texts))
	for i, text := range texts {
		messages[i] = newChatMessage("user", text, nil)
	}
	if !p.scan(w, r, key, messages) {
		return
	}
	for i := range texts {
		texts[i] = messages[i].Content
	}

	model, ok := p.findNamedModel(w, r, key, req.Model)
	if !ok {
		return
	}

	// Embeddings produce vectors, not tokens: the whole cost is on the
	// input side, so nothing is reserved for output.
	adm, ok := p.admitModel(w, r, key, model, estimateMessageTokens(messages), 0)
	if !ok {
		return
	}

	settled := false
	defer func() {
		if !settled {
			p.release(r.Context(), adm.Reservation)
		}
	}()

	start := time.Now()
	result, callErr := p.upstream.forwardEmbeddings(r.Context(), adm.Provider, model, req, texts, w)
	settled = p.settle(r, adm, result, callErr, time.Since(start))
}

// findNamedModel resolves the model a caller asked for by name, within
// what its key is allowed to reach.
func (p *Pipeline) findNamedModel(w http.ResponseWriter, r *http.Request, key types.VirtualKey, name string) (types.Model, bool) {
	models, err := p.providers.ListModelsForVirtualKey(r.Context(), key.ID)
	if err != nil {
		httpx.InternalError(w, err)
		return types.Model{}, false
	}
	for _, m := range models {
		if m.Name == name || m.ModelIdentifier == name {
			return m, true
		}
	}
	httpx.Error(w, http.StatusForbidden, i18n.KeyModelNotEnabled, "")
	return types.Model{}, false
}

// forwardEmbeddings relays an embeddings call.
//
// Only the providers that serve OpenAI's embeddings shape are wired up.
// Gemini and Bedrock both have embeddings APIs with entirely different
// request and response shapes, and Anthropic has none at all -- so
// rather than translate three more protocols here, or worse let the call
// fail somewhere unhelpful, this says which providers can serve the
// request and which cannot.
func (c *upstreamClient) forwardEmbeddings(ctx context.Context, p types.Provider, model types.Model, req embeddingsRequest, texts []string, w http.ResponseWriter) (outcome, error) {
	switch p.Kind {
	case types.ProviderKindOpenAICompatible, types.ProviderKindAzureOpenAI:
	default:
		return outcome{}, fmt.Errorf("gateway: provider kind %q does not serve embeddings", p.Kind)
	}

	endpoint, err := openAIEmbeddingsEndpoint(p, model)
	if err != nil {
		return outcome{}, err
	}

	body, err := req.withTexts(texts, model.ModelIdentifier)
	if err != nil {
		return outcome{}, err
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return outcome{}, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	applyOpenAIAuth(upstreamReq, p)

	resp, err := c.httpClient.Do(upstreamReq)
	if err != nil {
		return outcome{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return relayUpstreamError(w, resp)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return outcome{}, err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(raw); err != nil {
		return outcome{}, err
	}

	var parsed struct {
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil {
		return outcome{
			StatusSuccess: true,
			Usage:         &chatUsage{PromptTokens: parsed.Usage.PromptTokens},
		}, nil
	}
	return outcome{StatusSuccess: true}, nil
}
