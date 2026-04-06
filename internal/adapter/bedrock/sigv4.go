package bedrock

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// signV4 adds AWS SigV4 headers to an http.Request. This is a deliberately
// small implementation that covers exactly what Bedrock requires — POST
// requests to bedrock-runtime.{region}.amazonaws.com with a pre-computed
// request body. Implementing SigV4 in-tree keeps the binary free of the
// heavyweight AWS SDK, which would otherwise triple compile time and
// binary size.
func signV4(req *http.Request, body []byte, accessKey, secretKey, sessionToken, region, service string, now time.Time) {
	amzDate := now.UTC().Format("20060102T150405Z")
	dateStamp := now.UTC().Format("20060102")

	req.Header.Set("Host", req.Host)
	req.Header.Set("X-Amz-Date", amzDate)
	if sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", sessionToken)
	}
	payloadHash := hashHex(body)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	canonicalHeaders, signedHeaders := canonicalHeaderList(req)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req),
		canonicalQuery(req),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hashHex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(secretKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	auth := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", auth)
}

// canonicalHeaderList returns the canonical header string and the
// semicolon-separated signed-headers value. It lower-cases header names,
// trims values, and sorts lexicographically as required by SigV4.
func canonicalHeaderList(req *http.Request) (string, string) {
	names := make([]string, 0, len(req.Header)+1)
	seen := make(map[string]string, len(req.Header))
	for k, v := range req.Header {
		lk := strings.ToLower(k)
		seen[lk] = strings.Join(v, ",")
		names = append(names, lk)
	}
	// Host is required even though it is not in req.Header for client requests.
	if _, ok := seen["host"]; !ok {
		seen["host"] = req.Host
		names = append(names, "host")
	}
	sort.Strings(names)

	var b strings.Builder
	for _, n := range names {
		b.WriteString(n)
		b.WriteString(":")
		b.WriteString(strings.TrimSpace(seen[n]))
		b.WriteString("\n")
	}
	return b.String(), strings.Join(names, ";")
}

// canonicalURI returns the encoded path segment of the request URL. Bedrock
// only uses simple ASCII paths so aggressive percent-encoding is unnecessary.
func canonicalURI(req *http.Request) string {
	if req.URL.Path == "" {
		return "/"
	}
	return req.URL.EscapedPath()
}

// canonicalQuery returns the sorted, escaped query string. Bedrock runtime
// endpoints do not currently use query parameters but keeping this correct
// future-proofs the signer.
func canonicalQuery(req *http.Request) string {
	if req.URL.RawQuery == "" {
		return ""
	}
	q := req.URL.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("&")
		}
		for j, v := range q[k] {
			if j > 0 {
				b.WriteString("&")
			}
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(v)
		}
	}
	return b.String()
}

func deriveSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hashHex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
