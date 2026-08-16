package user

import "testing"

// The mask/merge pair is what makes stored credentials write-only. Both
// directions fail quietly when wrong: masking that misses a field leaks a
// password to every admin's browser, and merging that misses one replaces
// the stored password with the literal mask, breaking the channel at the
// next send rather than at save time.

func TestMaskChannelSecretsHidesCredentials(t *testing.T) {
	config := map[string]any{
		"host":         "smtp.example.com",
		"username":     "sender@example.com",
		"password":     "s3cr3t",
		"from_address": "sender@example.com",
	}

	masked := maskChannelSecrets(NotifyChannelEmail, config)

	if masked["password"] != maskedValue {
		t.Errorf("password = %v, want it masked", masked["password"])
	}
	for _, k := range []string{"host", "username", "from_address"} {
		if masked[k] != config[k] {
			t.Errorf("%s = %v, want the stored value %v", k, masked[k], config[k])
		}
	}
	// The caller's map is handed straight to the JSON encoder elsewhere;
	// masking in place would corrupt the config it was read from.
	if config["password"] != "s3cr3t" {
		t.Error("maskChannelSecrets mutated its argument")
	}
}

func TestMaskChannelSecretsCoversEveryKind(t *testing.T) {
	// A kind whose secret is not listed would publish that credential in
	// clear text, so the mapping is asserted rather than assumed.
	cases := []struct {
		kind   NotifyChannelKind
		secret string
	}{
		{NotifyChannelEmail, "password"},
		{NotifyChannelSMS, "access_key_secret"},
	}
	for _, c := range cases {
		masked := maskChannelSecrets(c.kind, map[string]any{c.secret: "real"})
		if masked[c.secret] != maskedValue {
			t.Errorf("%s: %s = %v, want it masked", c.kind, c.secret, masked[c.secret])
		}
	}
}

func TestMaskChannelSecretsLeavesAnEmptySecretAlone(t *testing.T) {
	// Masking an unset field would claim a credential exists and make the
	// form show "已保存，留空表示不修改" over nothing.
	masked := maskChannelSecrets(NotifyChannelEmail, map[string]any{"password": ""})
	if masked["password"] != "" {
		t.Errorf("password = %v, want it left empty", masked["password"])
	}
}

func TestMergeChannelSecretsKeepsStoredValue(t *testing.T) {
	stored := map[string]any{"password": "s3cr3t"}

	cases := map[string]map[string]any{
		"the mask is handed back": {"host": "smtp.example.com", "password": maskedValue},
		"the field is left blank": {"host": "smtp.example.com", "password": ""},
		"the field is absent":     {"host": "smtp.example.com"},
	}
	for name, incoming := range cases {
		merged := mergeChannelSecrets(NotifyChannelEmail, stored, incoming)
		if merged["password"] != "s3cr3t" {
			t.Errorf("%s: password = %v, want the stored secret preserved", name, merged["password"])
		}
	}
}

func TestMergeChannelSecretsAcceptsANewValue(t *testing.T) {
	merged := mergeChannelSecrets(NotifyChannelEmail,
		map[string]any{"password": "old"},
		map[string]any{"password": "new"})

	if merged["password"] != "new" {
		t.Errorf("password = %v, want the newly supplied secret", merged["password"])
	}
}

func TestMergeChannelSecretsDropsTheMaskWithNothingStored(t *testing.T) {
	// Nothing to fall back on, so the mask must not be saved as if it were
	// the password -- that would produce a channel that looks configured
	// and fails to authenticate.
	merged := mergeChannelSecrets(NotifyChannelEmail, nil, map[string]any{"password": maskedValue})

	if _, ok := merged["password"]; ok {
		t.Errorf("password = %v, want the mask dropped entirely", merged["password"])
	}
}
