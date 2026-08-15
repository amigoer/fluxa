package rules

import "regexp"

// bankCardShape matches a run of 13-19 digits, the range real bank card
// numbers fall in. Like ID cards, shape alone isn't enough -- see
// ValidateBankCard.
var bankCardShape = regexp.MustCompile(`\b\d{13,19}\b`)

// FindBankCards returns every substring of text that both looks like a
// bank card number and passes the Luhn check.
func FindBankCards(text string) []string {
	candidates := bankCardShape.FindAllString(text, -1)
	var matches []string
	for _, c := range candidates {
		if ValidateBankCard(c) {
			matches = append(matches, c)
		}
	}
	return matches
}

// ValidateBankCard checks a digit string against the Luhn algorithm,
// the check digit scheme real card numbers use.
func ValidateBankCard(digits string) bool {
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
