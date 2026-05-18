// request_log.go — per-call recorder that turns one in-flight HTTP
// request into a single store.RequestLog row. The recorder accumulates
// state as the handler progresses (parse → auth → resolve → upstream
// call) and is flushed via a deferred call at the end of the handler,
// so panics, early returns, and DLP blocks all still produce a row.
//
// The companion recordingResponseWriter wraps the live ResponseWriter
// so status code, response body, and the first-byte timestamp are
// captured without sprinkling explicit calls through every handler
// branch.

package api

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/amigoer/fluxa/internal/pricing"
	"github.com/amigoer/fluxa/internal/provider"
	"github.com/amigoer/fluxa/internal/store"
)

// MaxRequestLogBodyBytes caps how much of the request and response
// payloads we persist. Logs are debugging aids, not blobs: a multi-MB
// SSE stream would balloon the SQLite file and slow listing queries.
// Anything beyond the cap is truncated with an explicit marker so the
// reader knows the payload was clipped.
const MaxRequestLogBodyBytes = 256 << 10 // 256 KiB

// requestRecorder accumulates the state of one in-flight LLM call.
// One recorder is created per request after initial parsing succeeds;
// every handler branch mutates it via the small setter API. The
// matching recordingResponseWriter feeds it status + body bytes.
type requestRecorder struct {
	server         *Server
	started        time.Time
	firstByteAt    *time.Time
	endpoint       string
	method         string
	keyID          string
	modelRequested string
	modelResolved  string
	provider       string
	isStream       bool
	statusCode     int
	errMsg         string
	requestBody    []byte
	responseBody   bytes.Buffer
	clientIP       string
	userAgent      string
	extractUsage   usageExtractor // wire-format aware token parser, may be nil
}

// newRequestRecorder bootstraps a recorder with the metadata that is
// already known at handler entry. The rest is filled in by setters
// (setKey / setResolvedModel / setProvider / markError) and by the
// wrapped ResponseWriter (status, body, first-byte time).
func (s *Server) newRequestRecorder(r *http.Request, endpoint string, reqBody []byte, model string, isStream bool, extract usageExtractor) *requestRecorder {
	return &requestRecorder{
		server:         s,
		started:        time.Now(),
		endpoint:       endpoint,
		method:         r.Method,
		modelRequested: model,
		modelResolved:  model, // updated if the resolver rewrites the name
		isStream:       isStream,
		requestBody:    reqBody,
		clientIP:       clientIP(r),
		userAgent:      r.Header.Get("User-Agent"),
		extractUsage:   extract,
	}
}

func (rec *requestRecorder) setKey(keyID string) {
	if rec != nil {
		rec.keyID = keyID
	}
}

func (rec *requestRecorder) setProvider(name string) {
	if rec != nil {
		rec.provider = name
	}
}

func (rec *requestRecorder) setResolvedModel(model string) {
	if rec != nil {
		rec.modelResolved = model
	}
}

// markError stamps a short error message on the recorder so the list
// view has a tidy "error" column even when the full body is too large
// or too noisy to scan. Status and full body are still captured
// independently via the wrapped writer.
func (rec *requestRecorder) markError(err error) {
	if rec == nil || err == nil {
		return
	}
	var pErr *provider.Error
	if errors.As(err, &pErr) {
		rec.errMsg = pErr.Message
		return
	}
	rec.errMsg = err.Error()
}

// flush persists the recorder's accumulated state. Uses a background
// context so a canceled client request does not skip the audit write.
// Token extraction runs at flush time on the captured response body;
// it is best-effort and returns zeros for SSE-framed streams where the
// extractor cannot parse the concatenated chunks (a follow-up will
// handle stream-shaped token sums explicitly).
func (rec *requestRecorder) flush() {
	if rec == nil || rec.server == nil || rec.server.store == nil {
		return
	}
	completed := time.Now()
	latency := completed.Sub(rec.started)
	ttft := 0
	if rec.firstByteAt != nil {
		ttft = int(rec.firstByteAt.Sub(rec.started) / time.Millisecond)
	}

	var (
		prompt, completion, total int
		costUSD                   float64
	)
	if rec.extractUsage != nil && rec.responseBody.Len() > 0 {
		prompt, completion, total = rec.extractUsage(rec.responseBody.Bytes())
		if total == 0 {
			total = prompt + completion
		}
	}
	if total > 0 {
		modelForPricing := rec.modelResolved
		if modelForPricing == "" {
			modelForPricing = rec.modelRequested
		}
		costUSD = pricing.Cost(modelForPricing, prompt, completion)
	}

	status := rec.statusCode
	if status == 0 {
		status = http.StatusOK
	}

	log := store.RequestLog{
		VirtualKeyID:     rec.keyID,
		StartedAt:        rec.started,
		FirstByteAt:      rec.firstByteAt,
		CompletedAt:      completed,
		Endpoint:         rec.endpoint,
		Method:           rec.method,
		ModelRequested:   rec.modelRequested,
		ModelResolved:    rec.modelResolved,
		Provider:         rec.provider,
		IsStream:         rec.isStream,
		StatusCode:       status,
		Error:            rec.errMsg,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		CostUSD:          costUSD,
		LatencyMs:        int(latency / time.Millisecond),
		TTFTMs:           ttft,
		RequestBody:      string(truncateBody(rec.requestBody)),
		ResponseBody:     string(truncateBody(rec.responseBody.Bytes())),
		ClientIP:         rec.clientIP,
		UserAgent:        rec.userAgent,
	}
	if _, err := rec.server.store.InsertRequestLog(context.Background(), log); err != nil {
		rec.server.logger.Warn("insert request log", "err", err, "key", rec.keyID, "model", rec.modelRequested)
	}
}

// truncateBody keeps the first MaxRequestLogBodyBytes bytes of p and
// appends a marker when something was dropped. The marker is plain
// ASCII so JSON viewers render it without escape gymnastics.
func truncateBody(p []byte) []byte {
	if len(p) <= MaxRequestLogBodyBytes {
		return p
	}
	const suffix = "\n...[truncated]"
	out := make([]byte, 0, MaxRequestLogBodyBytes+len(suffix))
	out = append(out, p[:MaxRequestLogBodyBytes]...)
	out = append(out, suffix...)
	return out
}

// recordingResponseWriter wraps http.ResponseWriter to capture status
// + body bytes + first-byte time for the request log. Flusher is
// intentionally not implemented on this type so a streaming handler's
// `w.(http.Flusher)` assertion only succeeds when the underlying
// writer can actually flush; the recordingFlushWriter variant below
// is returned in that case.
type recordingResponseWriter struct {
	http.ResponseWriter
	rec        *requestRecorder
	headerSent bool
}

// WriteHeader records the first status the handler chose. Subsequent
// calls are forwarded so any double-WriteHeader warnings from the
// underlying implementation still surface.
func (rw *recordingResponseWriter) WriteHeader(status int) {
	if !rw.headerSent {
		rw.rec.statusCode = status
		rw.headerSent = true
	}
	rw.ResponseWriter.WriteHeader(status)
}

// Write tees the response body into the recorder up to the cap, then
// forwards verbatim to the underlying writer. The first non-empty
// Write also stamps firstByteAt so TTFT is accurate for streaming.
func (rw *recordingResponseWriter) Write(p []byte) (int, error) {
	if !rw.headerSent {
		rw.rec.statusCode = http.StatusOK
		rw.headerSent = true
	}
	if rw.rec.firstByteAt == nil && len(p) > 0 {
		t := time.Now()
		rw.rec.firstByteAt = &t
	}
	if rw.rec.responseBody.Len() < MaxRequestLogBodyBytes {
		remain := MaxRequestLogBodyBytes - rw.rec.responseBody.Len()
		if remain > len(p) {
			remain = len(p)
		}
		rw.rec.responseBody.Write(p[:remain])
	}
	return rw.ResponseWriter.Write(p)
}

// recordingFlushWriter is the Flusher-capable variant. Returned by
// wrapRecording when the underlying writer implements Flusher so
// streaming endpoints continue to flush after each SSE frame.
type recordingFlushWriter struct {
	recordingResponseWriter
	flusher http.Flusher
}

func (rw *recordingFlushWriter) Flush() { rw.flusher.Flush() }

// wrapRecording returns a writer that tees through to w while
// updating rec. The concrete return type implements http.Flusher only
// when the underlying writer does, so existing streaming handlers'
// capability checks continue to work without modification.
func wrapRecording(w http.ResponseWriter, rec *requestRecorder) http.ResponseWriter {
	base := recordingResponseWriter{ResponseWriter: w, rec: rec}
	if f, ok := w.(http.Flusher); ok {
		return &recordingFlushWriter{recordingResponseWriter: base, flusher: f}
	}
	return &base
}

// clientIP picks the best caller IP from a request, preferring the
// first hop in X-Forwarded-For when set (so operators behind a reverse
// proxy still get useful values). Returns RemoteAddr verbatim when no
// port is parseable so bare-IP test fixtures keep working.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// Compile-time assertion: recordingFlushWriter must satisfy Flusher.
var _ http.Flusher = (*recordingFlushWriter)(nil)
