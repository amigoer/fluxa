package bedrock

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// AWS event stream framing, minimal decoder.
//
// Bedrock's /converse-stream endpoint returns a sequence of binary frames
// using the application/vnd.amazon.eventstream content type. Each frame has
// the following layout:
//
//   total_length    (4 bytes, big-endian)
//   headers_length  (4 bytes, big-endian)
//   prelude_crc     (4 bytes, CRC32 of the preceding 8 bytes — unused here)
//   headers         (headers_length bytes)
//   payload         (total_length - headers_length - 16 bytes)
//   message_crc     (4 bytes)
//
// Headers are a sequence of:
//
//   name_length (1 byte) | name | value_type (1 byte) | value
//
// Bedrock only uses string (value_type == 7) headers, so we decode just
// those and skip anything else. We verify nothing against the CRCs because
// the HTTPS transport already guarantees integrity.

// eventFrame represents a single decoded EventStream frame.
type eventFrame struct {
	MessageType string // ":message-type" header, typically "event" or "exception"
	EventType   string // ":event-type" header, e.g. "contentBlockDelta"
	Payload     []byte // raw JSON payload
}

// readEventStream decodes successive EventStream frames from r and pushes
// them onto the provided callback. It returns when r is exhausted or the
// callback returns a non-nil error.
func readEventStream(r io.Reader, onFrame func(eventFrame) error) error {
	header := make([]byte, 12)
	for {
		if _, err := io.ReadFull(r, header[:12]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
		totalLen := binary.BigEndian.Uint32(header[0:4])
		headersLen := binary.BigEndian.Uint32(header[4:8])
		if totalLen < headersLen+16 {
			return fmt.Errorf("bedrock: invalid frame lengths total=%d headers=%d", totalLen, headersLen)
		}
		rest := make([]byte, totalLen-12)
		if _, err := io.ReadFull(r, rest); err != nil {
			return err
		}
		headerBytes := rest[:headersLen]
		payloadEnd := int(totalLen) - 12 - 4 // trailing message CRC
		payload := rest[headersLen:payloadEnd]

		frame := eventFrame{}
		if err := parseHeaders(headerBytes, &frame); err != nil {
			return err
		}
		frame.Payload = payload
		if err := onFrame(frame); err != nil {
			return err
		}
	}
}

// parseHeaders extracts the handful of headers the Bedrock adapter needs.
func parseHeaders(buf []byte, frame *eventFrame) error {
	for len(buf) > 0 {
		if len(buf) < 1 {
			return errors.New("bedrock: header name truncated")
		}
		nameLen := int(buf[0])
		buf = buf[1:]
		if len(buf) < nameLen+1 {
			return errors.New("bedrock: header name overrun")
		}
		name := string(buf[:nameLen])
		buf = buf[nameLen:]
		valueType := buf[0]
		buf = buf[1:]
		if valueType != 7 { // only strings are used by Bedrock
			return fmt.Errorf("bedrock: unsupported header value type %d", valueType)
		}
		if len(buf) < 2 {
			return errors.New("bedrock: header value length truncated")
		}
		valueLen := int(binary.BigEndian.Uint16(buf[0:2]))
		buf = buf[2:]
		if len(buf) < valueLen {
			return errors.New("bedrock: header value overrun")
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
