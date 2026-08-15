package rules

import "regexp"

// phonePattern matches an 11-digit Mainland China mobile number. Phone
// numbers have no check digit to validate against, unlike ID cards and
// bank cards, so shape matching is all there is.
var phonePattern = regexp.MustCompile(`\b1[3-9]\d{9}\b`)

// FindPhoneNumbers returns every substring of text that looks like a
// Mainland China mobile number.
func FindPhoneNumbers(text string) []string {
	return phonePattern.FindAllString(text, -1)
}
