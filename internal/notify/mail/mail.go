package mail

// Mail is one message's content.
//
// It lives in this package rather than in notify because notify imports
// this one to register the senders; notify aliases it back out so callers
// name it notify.Mail and never reach past the pluggable-channel layer.
//
// Text is not optional and not a courtesy: a message carrying only an
// HTML part is a well-known spam signal, and some clients still render
// the text one, so both go on the wire and the client picks.
type Mail struct {
	Subject string
	Text    string
	HTML    string
}
