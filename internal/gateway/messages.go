package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
)

// POST /v1/messages -- the Anthropic Messages API, inbound.
//
// This is what Claude Code and the Anthropic SDKs speak, and its absence
// is what kept them from being able to use Fluxa at all. The request is
// translated into the gateway's canonical shape on the way in and the
// answer translated back on the way out, which means an
// Anthropic-speaking client reaches whichever provider routing picked --
// including an OpenAI-compatible one. A deployment that never bought
// Anthropic capacity can still put Claude Code in front of what it did
// buy.
//
// It runs the same pipeline as /v1/chat/completions, deliberately: the
// same DLP scan, the same routing and ceilings, the same reservation and
// settlement, the same audit line. An endpoint that skipped any of those
// would be a way around all of them.
func (p *Pipeline) handleMessages(w http.ResponseWriter, r *http.Request) {
	key, ok := p.authenticate(w, r)
	if !ok {
		return
	}

	inbound, err := decodeMessagesRequest(r.Body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, err.Error())
		return
	}
	req, err := inbound.toChatRequest()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, err.Error())
		return
	}

	if !p.scan(w, r, key, req.Messages) {
		return
	}

	adm, ok := p.admit(w, r, key, req.Messages, req.MaxTokens)
	if !ok {
		return
	}

	settled := false
	defer func() {
		if !settled {
			p.release(r.Context(), adm.Reservation)
		}
	}()

	// The adapters answer in the canonical shape; this turns it back
	// into Anthropic's on the way to the caller.
	out := newMessagesWriter(w, req.Stream, inbound.Model)

	start := time.Now()
	result, callErr := p.upstream.forward(r.Context(), adm.Provider, adm.Model, req, out)
	if callErr == nil && result.StatusSuccess {
		if err := out.Finish(); err != nil {
			callErr = err
		}
	}
	settled = p.settle(r, adm, result, callErr, time.Since(start))
}

// randomishID gives a synthesised response the shape of id an Anthropic
// client expects when the provider that answered had no id of its own to
// report.
func randomishID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "fluxa"
	}
	return hex.EncodeToString(buf)
}
