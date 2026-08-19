package gateway

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
)

// GET /v1/models -- the catalogue as OpenAI serves it.
//
// Many clients probe this on startup to populate a model picker or just
// to check the base URL points at something real, and a gateway that
// 404s it looks broken before the first completion is ever attempted.
// The README's promise -- point an existing OpenAI client here and
// nothing else changes -- does not survive its absence.
//
// What it lists is what this key may call: the org's published models,
// narrowed to the key's own model scope. A caller should not learn the
// names of models it would be refused for asking about.

type modelCard struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type modelList struct {
	Object string      `json:"object"`
	Data   []modelCard `json:"data"`
}

func (p *Pipeline) handleModels(w http.ResponseWriter, r *http.Request) {
	key, ok := p.authenticate(w, r)
	if !ok {
		return
	}

	models, err := p.providers.ListModelsForVirtualKey(r.Context(), key.ID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	// data is built non-nil so an empty catalogue serializes as [] rather
	// than null; a client iterating the field should not have to special
	// case a deployment with nothing published yet.
	out := modelList{Object: "list", Data: make([]modelCard, 0, len(models))}
	for _, m := range models {
		out.Data = append(out.Data, modelCard{
			// The name an admin gave the model, which is what a caller
			// puts in "model" -- not the vendor's identifier, which is
			// an implementation detail of whichever provider serves it.
			ID:      m.Name,
			Object:  "model",
			Created: m.CreatedAt.Unix(),
			OwnedBy: string(m.ProviderKind),
		})
	}

	httpx.JSON(w, http.StatusOK, out)
}
