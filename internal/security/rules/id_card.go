// Package rules holds the built-in, checksummed sensitive-info
// detectors: a plain length/shape regex catches too many false
// positives (any 18-digit number looks like an ID card), so each type
// with a real check digit validates it too (DESIGN.md 7.3).
package rules

import (
	"regexp"
	"strconv"
	"strings"
)

// idCardShape matches an 18-digit Mainland China resident ID number
// shape (17 digits plus a check character that is a digit or X/x). This
// alone is not enough to call something a match -- see ValidateIDCard.
var idCardShape = regexp.MustCompile(`\b\d{17}[\dXx]\b`)

// idCardWeights and idCardCheckCodes implement the GB 11643-1999 check
// digit algorithm.
var idCardWeights = [17]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
var idCardCheckCodes = [11]byte{'1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'}

// FindIDCards returns every substring of text that both looks like an
// 18-digit ID number and passes its check digit, in order.
func FindIDCards(text string) []string {
	candidates := idCardShape.FindAllString(text, -1)
	var matches []string
	for _, c := range candidates {
		if ValidateIDCard(c) {
			matches = append(matches, c)
		}
	}
	return matches
}

// ValidateIDCard checks the GB 11643-1999 check digit of an 18-character
// Mainland China resident ID number.
func ValidateIDCard(id string) bool {
	if len(id) != 18 {
		return false
	}

	sum := 0
	for i := 0; i < 17; i++ {
		digit, err := strconv.Atoi(string(id[i]))
		if err != nil {
			return false
		}
		sum += digit * idCardWeights[i]
	}

	want := idCardCheckCodes[sum%11]
	got := strings.ToUpper(id[17:18])[0]
	return got == want
}
