package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Turning the gateway's OpenAI-shaped answer back into the Anthropic
// shape the caller of /v1/messages expects.
//
// This sits as a http.ResponseWriter in front of the real one so the
// upstream adapters stay exactly as they are: they know how to reach
// five vendors and answer in one shape, and nothing about them has to
// learn a second output format. Whatever they write here is translated
// on its way out.
//
// The streaming case cannot buffer -- that would undo the whole reason
// the gateway relays frame by frame -- so writes are scanned for
// complete SSE lines and each one is translated and flushed as it
// arrives. The most this ever holds is one partial line.

type messagesWriter struct {
	dst    http.ResponseWriter
	stream bool

	status  int
	headers http.Header

	// buf holds the non-streaming body until Finish, or the partial
	// trailing line while streaming.
	buf bytes.Buffer

	// upstreamFailed records that the adapter reported an error status,
	// in which case its body is relayed untranslated: an error is the
	// one thing worth passing through in whatever words the provider
	// used.
	upstreamFailed bool

	state anthropicEmitState
}

type anthropicEmitState struct {
	started       bool
	textOpen      bool
	blockIndex    int
	toolBlocks    map[int]int // OpenAI tool_calls index -> Anthropic block index
	openToolBlock int
	stopReason    string
	usage         chatUsage
	model         string
	id            string
}

func newMessagesWriter(dst http.ResponseWriter, stream bool, model string) *messagesWriter {
	return &messagesWriter{
		dst:     dst,
		stream:  stream,
		headers: http.Header{},
		state: anthropicEmitState{
			toolBlocks:    map[int]int{},
			openToolBlock: -1,
			model:         model,
			id:            "msg_" + randomishID(),
		},
	}
}

func (m *messagesWriter) Header() http.Header { return m.headers }

func (m *messagesWriter) WriteHeader(status int) {
	m.status = status
	if status >= 400 {
		m.upstreamFailed = true
		// An upstream error goes back as-is, so the real headers and
		// status are the ones to send.
		for k, v := range m.headers {
			m.dst.Header()[k] = v
		}
		m.dst.WriteHeader(status)
		return
	}
	if m.stream {
		m.dst.Header().Set("Content-Type", "text/event-stream")
		m.dst.Header().Set("Cache-Control", "no-cache")
	} else {
		m.dst.Header().Set("Content-Type", "application/json")
	}
	m.dst.WriteHeader(http.StatusOK)
}

func (m *messagesWriter) Write(p []byte) (int, error) {
	if m.upstreamFailed {
		return m.dst.Write(p)
	}
	if !m.stream {
		return m.buf.Write(p)
	}

	m.buf.Write(p)
	for {
		line, err := m.buf.ReadString('\n')
		if err != nil {
			// Partial line: put it back and wait for the rest.
			m.buf.Reset()
			m.buf.WriteString(line)
			break
		}
		if err := m.emitFromOpenAILine(line); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// Finish writes whatever the translation still owes: the whole body for
// a non-streaming call, or the closing events for a streamed one.
func (m *messagesWriter) Finish() error {
	if m.upstreamFailed {
		return nil
	}
	if !m.stream {
		return m.writeMessagesBody()
	}
	return m.closeStream()
}

// -- non-streaming ----------------------------------------------------------

func (m *messagesWriter) writeMessagesBody() error {
	var parsed struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content   *string          `json:"content"`
				ToolCalls []openAIToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage chatUsage `json:"usage"`
	}
	if err := json.Unmarshal(m.buf.Bytes(), &parsed); err != nil {
		return fmt.Errorf("gateway: messages: decode gateway response: %w", err)
	}

	content := []anthropicContent{}
	stopReason := "end_turn"
	if len(parsed.Choices) > 0 {
		choice := parsed.Choices[0]
		if choice.Message.Content != nil && *choice.Message.Content != "" {
			content = append(content, anthropicContent{Type: "text", Text: *choice.Message.Content})
		}
		for _, call := range choice.Message.ToolCalls {
			input := call.Function.Arguments
			if strings.TrimSpace(input) == "" {
				input = "{}"
			}
			content = append(content, anthropicContent{
				Type:  "tool_use",
				ID:    call.ID,
				Name:  call.Function.Name,
				Input: json.RawMessage(input),
			})
		}
		stopReason = openAIFinishToAnthropic(choice.FinishReason)
	}

	id := parsed.ID
	if id == "" {
		id = m.state.id
	}
	model := parsed.Model
	if model == "" {
		model = m.state.model
	}

	body, err := json.Marshal(map[string]any{
		"id":            id,
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       content,
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  parsed.Usage.PromptTokens,
			"output_tokens": parsed.Usage.CompletionTokens,
		},
	})
	if err != nil {
		return err
	}
	_, err = m.dst.Write(body)
	return err
}

// openAIFinishToAnthropic is the inverse of mapAnthropicStopReason.
func openAIFinishToAnthropic(reason string) string {
	switch reason {
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		// Anthropic has no equivalent; the turn did end, and saying so
		// is closer than inventing a reason the client has no branch for.
		return "end_turn"
	default:
		return "end_turn"
	}
}

// -- streaming --------------------------------------------------------------

func (m *messagesWriter) event(name string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(m.dst, "event: %s\ndata: %s\n\n", name, body); err != nil {
		return err
	}
	if flusher, ok := m.dst.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

// emitFromOpenAILine translates one line of the OpenAI stream the
// adapter produced.
func (m *messagesWriter) emitFromOpenAILine(line string) error {
	trimmed := strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(trimmed, dataPrefix) {
		return nil
	}
	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, dataPrefix))
	if payload == doneMarker {
		return nil // the closing events are written by closeStream
	}

	var chunk struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			FinishReason *string `json:"finish_reason"`
			Delta        struct {
				Content   *string          `json:"content"`
				ToolCalls []openAIToolCall `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *chatUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return nil
	}

	if chunk.Usage != nil {
		m.state.usage = *chunk.Usage
	}
	if chunk.ID != "" {
		m.state.id = chunk.ID
	}
	if chunk.Model != "" {
		m.state.model = chunk.Model
	}

	if err := m.ensureStarted(); err != nil {
		return err
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	choice := chunk.Choices[0]

	if choice.Delta.Content != nil && *choice.Delta.Content != "" {
		if err := m.openTextBlock(); err != nil {
			return err
		}
		if err := m.event("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": *choice.Delta.Content},
		}); err != nil {
			return err
		}
	}

	for _, call := range choice.Delta.ToolCalls {
		if err := m.emitToolCallDelta(call); err != nil {
			return err
		}
	}

	if choice.FinishReason != nil && *choice.FinishReason != "" {
		m.state.stopReason = openAIFinishToAnthropic(*choice.FinishReason)
	}
	return nil
}

func (m *messagesWriter) ensureStarted() error {
	if m.state.started {
		return nil
	}
	m.state.started = true
	return m.event("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            m.state.id,
			"type":          "message",
			"role":          "assistant",
			"model":         m.state.model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  m.state.usage.PromptTokens,
				"output_tokens": 0,
			},
		},
	})
}

// openTextBlock opens block 0 for text on first use. Text always takes
// block 0 here, so tool blocks start at 1 -- which keeps the indexes
// stable regardless of the order the OpenAI stream interleaved them.
func (m *messagesWriter) openTextBlock() error {
	if m.state.textOpen {
		return nil
	}
	m.state.textOpen = true
	m.state.blockIndex = 1
	return m.event("content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         0,
		"content_block": map[string]any{"type": "text", "text": ""},
	})
}

func (m *messagesWriter) emitToolCallDelta(call openAIToolCall) error {
	index := 0
	if call.Index != nil {
		index = *call.Index
	}

	block, known := m.state.toolBlocks[index]
	if !known {
		if m.state.blockIndex == 0 {
			m.state.blockIndex = 1 // reserve 0 for text even if none came
		}
		block = m.state.blockIndex
		m.state.blockIndex++
		m.state.toolBlocks[index] = block

		if m.state.openToolBlock >= 0 {
			if err := m.closeBlock(m.state.openToolBlock); err != nil {
				return err
			}
		}
		m.state.openToolBlock = block

		if err := m.event("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": block,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    call.ID,
				"name":  call.Function.Name,
				"input": map[string]any{},
			},
		}); err != nil {
			return err
		}
	}

	if call.Function.Arguments == "" {
		return nil
	}
	return m.event("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": block,
		"delta": map[string]any{"type": "input_json_delta", "partial_json": call.Function.Arguments},
	})
}

func (m *messagesWriter) closeBlock(index int) error {
	return m.event("content_block_stop", map[string]any{"type": "content_block_stop", "index": index})
}

func (m *messagesWriter) closeStream() error {
	if err := m.ensureStarted(); err != nil {
		return err
	}
	if m.state.textOpen {
		if err := m.closeBlock(0); err != nil {
			return err
		}
	}
	if m.state.openToolBlock >= 0 {
		if err := m.closeBlock(m.state.openToolBlock); err != nil {
			return err
		}
	}

	stopReason := m.state.stopReason
	if stopReason == "" {
		stopReason = "end_turn"
	}
	if err := m.event("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": map[string]any{"output_tokens": m.state.usage.CompletionTokens},
	}); err != nil {
		return err
	}
	return m.event("message_stop", map[string]any{"type": "message_stop"})
}
