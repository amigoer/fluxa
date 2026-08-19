package gateway

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// dataPrefix is the only SSE field the gateway reads. Comments (":"),
// event names and retry hints are relayed untouched like everything
// else.
const dataPrefix = "data: "

// doneMarker ends an OpenAI-compatible stream.
const doneMarker = "[DONE]"

// streamFrame is the part of one chunk the gateway cares about. Usage is
// a pointer so a frame that omits it is distinguishable from one
// reporting zeros, and Choices stays raw because nothing here reads
// inside it -- only whether it is empty, which is what marks the
// usage-only frame at the end of the stream.
type streamFrame struct {
	Usage   *chatUsage        `json:"usage"`
	Choices []json.RawMessage `json:"choices"`
}

// sseRelay copies an SSE stream through to the caller while reading the
// token usage out of it on the way past.
//
// Streaming used to be relayed with a plain io.Copy, so real usage never
// came back and every streamed call -- which is to say most calls -- was
// billed against a character-count estimate instead. Reading it requires
// looking at the bytes, but not holding them: this forwards each line as
// it completes and flushes, so the caller sees the stream at the same
// rate it would from a straight copy. The most it ever holds is one
// frame.
//
// It is written against the SSE framing rather than against any one
// vendor's chunk shape, so an adapter for a provider that speaks
// OpenAI-style SSE can reuse it as-is.
type sseRelay struct {
	// DropUsageOnlyFrame suppresses the trailing frame that carries
	// usage and no choices. The gateway asks providers for that frame
	// whether or not the caller did; when the caller did not, relaying
	// it would hand them a chunk they never asked for and, for a client
	// that reaches straight into choices[0], one it may not survive.
	DropUsageOnlyFrame bool

	// Usage is the last usage the stream reported, nil if it reported
	// none.
	Usage *chatUsage
}

// Run relays src to dst until the stream ends, recording usage as it
// goes. A write failure to dst stops the relay; a read failure from src
// mid-stream does too, and both are returned.
func (s *sseRelay) Run(dst io.Writer, src io.Reader) error {
	// A bufio.Reader rather than a Scanner: a Scanner caps how long a
	// line may be and drops the stream when one exceeds it, and losing
	// bytes the caller is owed is not an acceptable failure mode for a
	// relay. Frames are bounded in practice by the provider.
	reader := bufio.NewReader(src)
	skipNextBlank := false

	for {
		line, readErr := reader.ReadString('\n')

		if line != "" {
			forward, blank := s.inspect(line)
			if skipNextBlank && blank {
				skipNextBlank = false
				forward = false
			} else if !forward {
				// The dropped frame's terminating blank line goes with it.
				skipNextBlank = true
			}
			if forward {
				if _, err := io.WriteString(dst, line); err != nil {
					return err
				}
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

// inspect records what a line reports and decides whether it goes on to
// the caller. blank reports whether the line is an event separator,
// which the caller uses to drop the separator belonging to a dropped
// frame.
func (s *sseRelay) inspect(line string) (forward, blank bool) {
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed == "" {
		return true, true
	}
	if !strings.HasPrefix(trimmed, dataPrefix) {
		return true, false
	}

	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, dataPrefix))
	if payload == doneMarker {
		return true, false
	}

	var frame streamFrame
	if err := json.Unmarshal([]byte(payload), &frame); err != nil {
		// Not a shape this understands. Relaying is always safe; only
		// the usage reading is lost, and that falls back to an estimate.
		return true, false
	}
	if frame.Usage == nil {
		return true, false
	}

	s.Usage = frame.Usage

	// Providers differ on where they put usage: some send a final frame
	// carrying only usage, others attach it to every chunk. Only the
	// former is the frame the gateway asked for and may need to hide.
	if s.DropUsageOnlyFrame && len(frame.Choices) == 0 {
		return false, false
	}
	return true, false
}

// eventPrefix names an SSE event. OpenAI-compatible streams don't use
// it; Anthropic's does, and it is what says which shape the data line
// carries.
const eventPrefix = "event:"

// scanSSEEvents reads an SSE stream and calls fn once per complete
// event, with the event name (empty when the stream doesn't use them)
// and the data payload.
//
// This is the reading half that sseRelay's forwarding half cannot serve:
// an adapter translating one vendor's stream into OpenAI's does not
// relay the bytes it reads, it emits different ones. Both halves work
// off SSE framing rather than any vendor's chunk shape, so both are
// reusable by the next adapter.
//
// fn returning an error stops the scan and is returned as-is.
func scanSSEEvents(src io.Reader, fn func(event, data string) error) error {
	reader := bufio.NewReader(src)
	var event, data string

	flush := func() error {
		if data == "" {
			event = ""
			return nil
		}
		err := fn(event, data)
		event, data = "", ""
		return err
	}

	for {
		line, readErr := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")

		switch {
		case trimmed == "":
			if line != "" || data != "" {
				if err := flush(); err != nil {
					return err
				}
			}
		case strings.HasPrefix(trimmed, eventPrefix):
			event = strings.TrimSpace(strings.TrimPrefix(trimmed, eventPrefix))
		case strings.HasPrefix(trimmed, "data:"):
			// Multiple data lines in one event concatenate with newlines,
			// per the SSE spec.
			payload := strings.TrimPrefix(strings.TrimPrefix(trimmed, "data:"), " ")
			if data == "" {
				data = payload
			} else {
				data += "\n" + payload
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return flush()
			}
			return readErr
		}
	}
}

// writeSSEData writes one OpenAI-format SSE frame and flushes it.
func writeSSEData(w io.Writer, payload []byte) error {
	if _, err := io.WriteString(w, dataPrefix); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n\n")
	return err
}
