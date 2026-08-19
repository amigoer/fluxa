package gateway

import (
	"net/http"
	"strings"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// longInputTokenThreshold is what pushes a request into the "长文本"
// routing condition from the mockup automatically, without the caller
// having to tag it.
const longInputTokenThreshold = 50_000

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func memberIDOrEmpty(key types.VirtualKey) string {
	if key.OwnerMemberID != nil {
		return *key.OwnerMemberID
	}
	return ""
}

func memberIDPtr(key types.VirtualKey) *string {
	return key.OwnerMemberID
}

func modelInScope(key types.VirtualKey, modelID string) bool {
	if key.ModelScope == nil {
		return true // nil scope means every enabled model is allowed
	}
	for _, id := range key.ModelScope {
		if id == modelID {
			return true
		}
	}
	return false
}

// latinCharsPerToken is roughly how many characters of Latin-script text
// one token covers.
const latinCharsPerToken = 4

// estimateTokens approximates a piece of text's token count without
// running a tokenizer.
//
// It counts CJK characters separately because dividing everything by
// four is an English-only rule: a Chinese character is on the order of
// one token, so a Chinese prompt came out two to four times under. That
// number decides whether a call is admitted against its budget, and on
// a provider that reports no usage it is also what the call is billed
// on, so a systematic undercount there is the same failure as billing
// nothing at all -- just slower.
//
// One token per CJK character is the conservative end of the real range
// (roughly one to one and a half characters per token). Erring high is
// the right direction here for the same reason it is in the reservation
// estimate: the number gates spending.
func estimateTokens(text string) int {
	cjk, rest := 0, 0
	for _, r := range text {
		if isCJK(r) {
			cjk++
		} else {
			rest++
		}
	}
	return cjk + rest/latinCharsPerToken + 1
}

// isCJK reports whether r belongs to a script whose characters carry
// roughly a token each rather than a quarter of one: Han, kana, Hangul,
// and the fullwidth punctuation that comes with them.
func isCJK(r rune) bool {
	switch {
	case r >= 0x3000 && r <= 0x303F: // CJK symbols and punctuation
		return true
	case r >= 0x3040 && r <= 0x30FF: // hiragana, katakana
		return true
	case r >= 0x3400 && r <= 0x4DBF: // Han extension A
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // Han
		return true
	case r >= 0xAC00 && r <= 0xD7AF: // Hangul syllables
		return true
	case r >= 0xF900 && r <= 0xFAFF: // Han compatibility
		return true
	case r >= 0xFF00 && r <= 0xFF60: // fullwidth forms
		return true
	case r >= 0x20000 && r <= 0x2FA1F: // Han extensions B and beyond
		return true
	default:
		return false
	}
}

// estimateMessageTokens approximates a whole request's input size the
// same rough way estimateTokens does for one piece of text.
func estimateMessageTokens(messages []chatMessage) int {
	total := 0
	for _, m := range messages {
		total += estimateTokens(m.Content)
	}
	return total
}

func routingCondition(r *http.Request, estimatedInputTokens int) string {
	if estimatedInputTokens > longInputTokenThreshold {
		return "长文本（>50K）"
	}
	if hint := r.Header.Get("X-Fluxa-Route-Condition"); hint != "" {
		return hint
	}
	return "默认"
}
