package handler

import (
	"testing"

	"github.com/amigoer/fluxa/internal/provider/types"
)

func TestMaskProviderSecretsRedactsCredentialsAndKeepsSettings(t *testing.T) {
	p := types.Provider{
		Name:   "OpenAI 主账号",
		Kind:   types.ProviderKindOpenAICompatible,
		Config: map[string]any{"base_url": "https://api.openai.com/v1", "api_key": "sk-live-abcdef"},
	}

	masked := maskProviderSecrets(p)

	if masked.Config["api_key"] != maskedValue {
		t.Errorf("api_key = %v, want it redacted", masked.Config["api_key"])
	}
	if masked.Config["base_url"] != "https://api.openai.com/v1" {
		t.Errorf("base_url = %v, want it left readable", masked.Config["base_url"])
	}
}

// The gateway reads the same config to authenticate upstream. Masking a
// copy for the client must not reach back into what it was copied from.
func TestMaskProviderSecretsDoesNotMutateTheOriginal(t *testing.T) {
	config := map[string]any{"api_key": "sk-live-abcdef"}
	_ = maskProviderSecrets(types.Provider{Config: config})

	if config["api_key"] != "sk-live-abcdef" {
		t.Fatalf("api_key = %v, want the stored value untouched", config["api_key"])
	}
}

func TestMaskProviderSecretsLeavesAnEmptyCredentialAlone(t *testing.T) {
	masked := maskProviderSecrets(types.Provider{Config: map[string]any{"api_key": ""}})
	if masked.Config["api_key"] != "" {
		t.Errorf("api_key = %v, want \"\" rather than a mask over nothing", masked.Config["api_key"])
	}
}

func TestMaskProviderListSecretsCoversEveryRow(t *testing.T) {
	out := maskProviderListSecrets([]types.Provider{
		{Config: map[string]any{"api_key": "sk-1"}},
		{Config: map[string]any{"api_key": "sk-2"}},
		{Config: nil},
	})
	for i, p := range out[:2] {
		if p.Config["api_key"] != maskedValue {
			t.Errorf("row %d api_key = %v, want it redacted", i, p.Config["api_key"])
		}
	}
}

// Every kind the schema accepts now has a translation behind it. A kind
// it does not accept still has to be refused at the door rather than
// reaching a CHECK constraint violation dressed up as a 500.
func TestOnlyImplementedKindsAreAccepted(t *testing.T) {
	for kind, want := range map[types.ProviderKind]bool{
		types.ProviderKindOpenAICompatible: true,
		types.ProviderKindAnthropic:        true,
		types.ProviderKindAzureOpenAI:      true,
		types.ProviderKindGemini:           true,
		types.ProviderKindBedrock:          true,
		types.ProviderKind("alibaba"):      false,
		types.ProviderKind("cohere"):       false,
		types.ProviderKind(""):             false,
	} {
		if got := kind.Implemented(); got != want {
			t.Errorf("%q.Implemented() = %v, want %v", kind, got, want)
		}
	}
}
