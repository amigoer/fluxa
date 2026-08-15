package rules

import "testing"

func TestValidateIDCard(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"11010519491231002X", true},  // commonly used example ID, well-known valid check digit
		{"11010519491231002Y", false}, // wrong check character
		{"1101051949123100", false},   // too short
		{"not-an-id-number1", false},
	}
	for _, c := range cases {
		if got := ValidateIDCard(c.id); got != c.want {
			t.Errorf("ValidateIDCard(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

func TestFindIDCards(t *testing.T) {
	matches := FindIDCards("我的身份证号是11010519491231002X，请核实")
	if len(matches) != 1 || matches[0] != "11010519491231002X" {
		t.Errorf("FindIDCards() = %v, want [11010519491231002X]", matches)
	}

	if matches := FindIDCards("这只是一个18位数字但不是身份证110000000000000000"); len(matches) != 0 {
		t.Errorf("FindIDCards() should reject a shape match that fails its check digit, got %v", matches)
	}
}

func TestValidateBankCard(t *testing.T) {
	cases := []struct {
		number string
		want   bool
	}{
		{"4111111111111111", true},  // well-known Luhn-valid test card number
		{"4111111111111112", false}, // last digit flipped, fails Luhn
	}
	for _, c := range cases {
		if got := ValidateBankCard(c.number); got != c.want {
			t.Errorf("ValidateBankCard(%q) = %v, want %v", c.number, got, c.want)
		}
	}
}

func TestFindPhoneNumbers(t *testing.T) {
	matches := FindPhoneNumbers("call me at 13812345678 thanks, or 021-12345678 (landline, not a match)")
	if len(matches) != 1 || matches[0] != "13812345678" {
		t.Errorf("FindPhoneNumbers() = %v, want [13812345678]", matches)
	}
}
