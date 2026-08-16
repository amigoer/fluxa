package notify

import "testing"

// Configured is the gate on enabling a channel, and everything downstream
// -- including whether the login page offers local accounts at all --
// treats "enabled" as a promise that a code will actually arrive.

func TestConfiguredRequiresEveryMandatoryField(t *testing.T) {
	full := map[string]any{"host": "smtp.example.com", "port": "465", "from_address": "a@example.com"}
	if !Configured("smtp", full) {
		t.Fatal("a complete smtp config was rejected")
	}

	for missing := range full {
		partial := map[string]any{}
		for k, v := range full {
			if k != missing {
				partial[k] = v
			}
		}
		if Configured("smtp", partial) {
			t.Errorf("smtp config without %q was accepted", missing)
		}
	}
}

func TestConfiguredRejectsBlankValues(t *testing.T) {
	// A whitespace-only host reaches the relay as a dial to nowhere, so it
	// is no more usable than an absent one.
	config := map[string]any{"host": "   ", "port": "465", "from_address": "a@example.com"}
	if Configured("smtp", config) {
		t.Error("a whitespace-only host was accepted")
	}
}

func TestConfiguredCredentialsAreOptionalForSMTP(t *testing.T) {
	// Plenty of internal relays accept unauthenticated submission from
	// inside the network; demanding a password would lock those out.
	config := map[string]any{"host": "smtp.internal", "port": "25", "from_address": "a@example.com"}
	if !Configured("smtp", config) {
		t.Error("smtp without credentials was rejected")
	}
}

func TestConfiguredRequiresAliyunCredentials(t *testing.T) {
	config := map[string]any{
		"access_key_id":     "id",
		"access_key_secret": "secret",
		"sign_name":         "Fluxa",
		"template_code":     "SMS_1",
	}
	if !Configured("aliyun_sms", config) {
		t.Fatal("a complete aliyun config was rejected")
	}

	delete(config, "access_key_secret")
	if Configured("aliyun_sms", config) {
		t.Error("aliyun config without its secret was accepted")
	}
}

func TestConfiguredRejectsUnknownProvider(t *testing.T) {
	// Fail closed: an unrecognised provider has no send path behind it, so
	// enabling it could only ever produce a channel that cannot deliver.
	if Configured("carrier-pigeon", map[string]any{"anything": "set"}) {
		t.Error("an unknown provider was reported as configured")
	}
	if Configured("", nil) {
		t.Error("an empty provider was reported as configured")
	}
}
