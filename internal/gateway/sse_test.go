package gateway

import (
	"strings"
	"testing"
)

// chunk is one SSE event as a provider writes it.
func chunk(payload string) string { return "data: " + payload + "\n\n" }

const (
	deltaFrame = `{"id":"c1","object":"chat.completion.chunk","choices":[{"delta":{"content":"hi"}}]}`
	usageFrame = `{"id":"c1","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":1200,"completion_tokens":340}}`
)

func TestRelayReadsUsageOutOfTheFinalFrame(t *testing.T) {
	stream := chunk(deltaFrame) + chunk(usageFrame) + chunk("[DONE]")

	relay := &sseRelay{}
	var out strings.Builder
	if err := relay.Run(&out, strings.NewReader(stream)); err != nil {
		t.Fatalf("relay: %v", err)
	}

	if relay.Usage == nil {
		t.Fatal("no usage read from the stream")
	}
	if relay.Usage.PromptTokens != 1200 || relay.Usage.CompletionTokens != 340 {
		t.Errorf("usage = %+v, want 1200/340", *relay.Usage)
	}
}

// The gateway asks for the usage frame whether or not the caller did. A
// caller that did not ask must not see it: a client reading choices[0]
// off every chunk would trip over one that has none.
func TestRelayHidesTheUsageFrameTheCallerDidNotAskFor(t *testing.T) {
	stream := chunk(deltaFrame) + chunk(usageFrame) + chunk("[DONE]")

	relay := &sseRelay{DropUsageOnlyFrame: true}
	var out strings.Builder
	if err := relay.Run(&out, strings.NewReader(stream)); err != nil {
		t.Fatalf("relay: %v", err)
	}

	if strings.Contains(out.String(), "usage") {
		t.Errorf("the usage frame reached the caller:\n%s", out.String())
	}
	if relay.Usage == nil {
		t.Error("dropping the frame also dropped the reading of it")
	}
	if got, want := out.String(), chunk(deltaFrame)+chunk("[DONE]"); got != want {
		t.Errorf("relayed:\n%q\nwant:\n%q", got, want)
	}
}

func TestRelayKeepsTheUsageFrameTheCallerAskedFor(t *testing.T) {
	stream := chunk(deltaFrame) + chunk(usageFrame) + chunk("[DONE]")

	relay := &sseRelay{DropUsageOnlyFrame: false}
	var out strings.Builder
	if err := relay.Run(&out, strings.NewReader(stream)); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if out.String() != stream {
		t.Errorf("relayed:\n%q\nwant the stream unchanged:\n%q", out.String(), stream)
	}
}

// Some providers attach usage to every chunk rather than sending a
// separate final frame. Those chunks carry content and must go through.
func TestRelayKeepsUsageThatRidesAlongWithContent(t *testing.T) {
	inline := `{"choices":[{"delta":{"content":"hi"}}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`
	stream := chunk(inline) + chunk("[DONE]")

	relay := &sseRelay{DropUsageOnlyFrame: true}
	var out strings.Builder
	if err := relay.Run(&out, strings.NewReader(stream)); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if out.String() != stream {
		t.Errorf("a content-carrying chunk was dropped:\n%q", out.String())
	}
	if relay.Usage == nil || relay.Usage.PromptTokens != 10 {
		t.Errorf("usage = %+v, want it read from the inline frame", relay.Usage)
	}
}

// Anything the relay does not understand goes through untouched --
// comments, event names, a payload that is not the shape it expects.
// Losing usage costs an estimate; corrupting the stream costs the call.
func TestRelayPassesThroughWhatItDoesNotUnderstand(t *testing.T) {
	stream := ": keep-alive\n\n" +
		"event: message\n" +
		chunk(`{"not":"a chunk shape we know"}`) +
		"data: not json at all\n\n" +
		chunk("[DONE]")

	relay := &sseRelay{DropUsageOnlyFrame: true}
	var out strings.Builder
	if err := relay.Run(&out, strings.NewReader(stream)); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if out.String() != stream {
		t.Errorf("relayed:\n%q\nwant:\n%q", out.String(), stream)
	}
	if relay.Usage != nil {
		t.Errorf("usage = %+v, want none", *relay.Usage)
	}
}

func TestRelayHandlesAStreamThatEndsWithoutATrailingNewline(t *testing.T) {
	stream := chunk(deltaFrame) + "data: [DONE]"

	relay := &sseRelay{}
	var out strings.Builder
	if err := relay.Run(&out, strings.NewReader(stream)); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if out.String() != stream {
		t.Errorf("relayed:\n%q\nwant:\n%q", out.String(), stream)
	}
}

func TestRelayPreservesCarriageReturns(t *testing.T) {
	stream := "data: " + deltaFrame + "\r\n\r\ndata: [DONE]\r\n\r\n"

	relay := &sseRelay{DropUsageOnlyFrame: true}
	var out strings.Builder
	if err := relay.Run(&out, strings.NewReader(stream)); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if out.String() != stream {
		t.Errorf("relayed:\n%q\nwant the bytes unchanged:\n%q", out.String(), stream)
	}
}
