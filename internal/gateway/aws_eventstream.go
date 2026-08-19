package gateway

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// AWS event-stream framing, minimal decoder.
//
// Bedrock's converse-stream endpoint answers in
// application/vnd.amazon.eventstream rather than SSE. Each frame is:
//
//	total_length   (4 bytes, big-endian)
//	headers_length (4 bytes, big-endian)
//	prelude_crc    (4 bytes)
//	headers        (headers_length bytes)
//	payload        (total_length - headers_length - 16 bytes)
//	message_crc    (4 bytes)
//
// and each header is name_length(1) | name | value_type(1) | value.
// CRCs are not checked: TLS already guarantees integrity, and a mismatch
// here would mean a bug in this decoder rather than a corrupted wire.
//
// Ported from f5b192a with one fix: header value types other than string
// are now skipped by their real width instead of failing the stream.
// Bedrock's frames are string-headed today, but a stream that dies on
// the first timestamp header AWS adds is a stream that dies in
// production and nowhere else.

type eventFrame struct {
	MessageType string // ":message-type", e.g. "event" or "exception"
	EventType   string // ":event-type", e.g. "contentBlockDelta"
	Payload     []byte // raw JSON
}

// readEventStream decodes frames from r until it is exhausted or onFrame
// returns an error.
func readEventStream(r io.Reader, onFrame func(eventFrame) error) error {
	prelude := make([]byte, 12)
	for {
		if _, err := io.ReadFull(r, prelude); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
		totalLen := binary.BigEndian.Uint32(prelude[0:4])
		headersLen := binary.BigEndian.Uint32(prelude[4:8])
		if totalLen < headersLen+16 {
			return fmt.Errorf("gateway: bedrock: invalid frame lengths total=%d headers=%d", totalLen, headersLen)
		}

		rest := make([]byte, totalLen-12)
		if _, err := io.ReadFull(r, rest); err != nil {
			return err
		}

		frame := eventFrame{}
		if err := parseEventHeaders(rest[:headersLen], &frame); err != nil {
			return err
		}
		frame.Payload = rest[headersLen : int(totalLen)-12-4]
		if err := onFrame(frame); err != nil {
			return err
		}
	}
}

// eventHeaderValueWidth returns the fixed width of a header value type,
// or -1 for the two length-prefixed ones.
func eventHeaderValueWidth(valueType byte) int {
	switch valueType {
	case 0, 1: // bool true / false, no payload
		return 0
	case 2: // byte
		return 1
	case 3: // short
		return 2
	case 4: // integer
		return 4
	case 5, 8: // long, timestamp
		return 8
	case 9: // uuid
		return 16
	case 6, 7: // byte array, string -- 2-byte length prefix
		return -1
	default:
		return -2
	}
}

func parseEventHeaders(buf []byte, frame *eventFrame) error {
	for len(buf) > 0 {
		nameLen := int(buf[0])
		buf = buf[1:]
		if len(buf) < nameLen+1 {
			return errors.New("gateway: bedrock: header name overruns the frame")
		}
		name := string(buf[:nameLen])
		buf = buf[nameLen:]

		valueType := buf[0]
		buf = buf[1:]

		width := eventHeaderValueWidth(valueType)
		switch {
		case width == -2:
			return fmt.Errorf("gateway: bedrock: unknown header value type %d", valueType)
		case width >= 0:
			if len(buf) < width {
				return errors.New("gateway: bedrock: header value overruns the frame")
			}
			buf = buf[width:]
			continue
		}

		if len(buf) < 2 {
			return errors.New("gateway: bedrock: header value length truncated")
		}
		valueLen := int(binary.BigEndian.Uint16(buf[0:2]))
		buf = buf[2:]
		if len(buf) < valueLen {
			return errors.New("gateway: bedrock: header value overruns the frame")
		}
		value := string(buf[:valueLen])
		buf = buf[valueLen:]

		switch name {
		case ":message-type":
			frame.MessageType = value
		case ":event-type":
			frame.EventType = value
		}
	}
	return nil
}
