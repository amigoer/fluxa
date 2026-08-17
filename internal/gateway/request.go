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

func joinMessageContent(messages []chatMessage) string {
	parts := make([]string, len(messages))
	for i, m := range messages {
		parts[i] = m.Content
	}
	return strings.Join(parts, "\n")
}

// replaceMessageContent re-splits the scanned, masked text back across
// the original messages. Because DLP replacement never changes line
// count (matched substrings are replaced in place, not the newlines
// joining messages), splitting on "\n" the same way joinMessageContent
// joined always yields the right number of parts back.
func replaceMessageContent(messages []chatMessage, maskedJoined string) []chatMessage {
	parts := strings.Split(maskedJoined, "\n")
	if len(parts) != len(messages) {
		return messages
	}
	out := make([]chatMessage, len(messages))
	for i, m := range messages {
		out[i] = chatMessage{Role: m.Role, Content: parts[i]}
	}
	return out
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

// estimateTokens is a rough, fast approximation (roughly 4 characters
// per token for English; conservative enough for routing/cost-ceiling
// decisions, which only need to be in the right ballpark, not exact).
func estimateTokens(text string) int {
	return len([]rune(text))/4 + 1
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
