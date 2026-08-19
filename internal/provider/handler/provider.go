package handler

import (
	"errors"
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/provider/service"
	"github.com/amigoer/fluxa/internal/provider/types"
	"github.com/amigoer/fluxa/internal/rbac"
)

// maskedValue is what a credential reads as once it has been stored.
// The console already tells an admin as much ("凭证保存后只回显掩码"); this
// is what makes that true.
const maskedValue = "****"

// providerSecretConfigKeys names the config entries that are credentials
// rather than settings. Everything else in config -- base_url, region,
// deployment names -- is operational detail an admin is meant to read
// back, so only these are redacted.
//
// A kind added later brings its own credential field names; add them
// here at the same time. Nothing enforces that, so the list is
// deliberately broader than the one kind the gateway can forward to
// today: a name that turns out to be unused costs nothing, and a name
// that turns out to be missing costs a plaintext credential.
var providerSecretConfigKeys = []string{
	"api_key",
	"app_secret",
	"access_key_secret",
	"secret_access_key",
	"password",
	"token",
}

// maskProviderSecrets returns a copy of p with its credentials redacted,
// for handing to a client.
//
// This is not decoration. providers.config holds upstream API keys in
// plaintext (a deliberate v1 tradeoff for a single-tenant deployment,
// see the schema), and this endpoint used to serialize the map whole --
// so every session holding provider.view, which the console fetches on
// load, received the org's real upstream keys and kept them in browser
// memory. provider.view and provider.manage_credentials are separate
// permissions precisely so that reading the provider list is not the
// same as holding its credentials; without this the split did not exist
// in practice.
//
// The gateway is unaffected: it reads config through the service, not
// through here.
func maskProviderSecrets(p types.Provider) types.Provider {
	if p.Config == nil {
		return p
	}
	config := make(map[string]any, len(p.Config))
	for k, v := range p.Config {
		config[k] = v
	}
	for _, k := range providerSecretConfigKeys {
		if s, _ := config[k].(string); s != "" {
			config[k] = maskedValue
		}
	}
	p.Config = config
	return p
}

func maskProviderListSecrets(providers []types.Provider) []types.Provider {
	out := make([]types.Provider, len(providers))
	for i, p := range providers {
		out[i] = maskProviderSecrets(p)
	}
	return out
}

func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	providers, err := h.service.ListProviders(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, maskProviderListSecrets(providers))
}

func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var p types.Provider
	if !decodeJSON(w, r, &p) {
		return
	}
	p.OrgID = principal.OrgID
	created, err := h.service.CreateProvider(r.Context(), p)
	if errors.Is(err, service.ErrProviderKindUnsupported) {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyProviderKindUnsupported, err.Error())
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, maskProviderSecrets(created))
}
