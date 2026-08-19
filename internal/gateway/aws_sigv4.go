package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AWS Signature Version 4, in-tree.
//
// This covers exactly what Bedrock needs -- a POST to
// bedrock-runtime.{region}.amazonaws.com with a body already in hand --
// rather than pulling in the AWS SDK, which for one signing function
// would add a large dependency tree to a binary whose whole deployment
// story is that it is one file.
//
// Ported from the implementation removed in f5b192a, with two fixes: the
// canonical query string now percent-encodes and sorts values (the old
// one emitted them raw, which signs a different string than the one sent
// for any value needing an escape), and the host is taken from the URL
// rather than req.Host, because that is what Go actually puts on the
// wire and a signature over a different value is rejected.

func signV4(req *http.Request, body []byte, accessKey, secretKey, sessionToken, region, service string, now time.Time) {
	amzDate := now.UTC().Format("20060102T150405Z")
	dateStamp := now.UTC().Format("20060102")

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

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, signedHeaders, signature))
}

// canonicalHeaderList returns the canonical header block and the
// signed-headers list: names lower-cased, values trimmed, sorted.
//
// host is added from the URL because Go sends it from there and never
// from req.Header, so signing anything else produces a signature over a
// request that was not the one sent.
func canonicalHeaderList(req *http.Request) (string, string) {
	values := make(map[string]string, len(req.Header)+1)
	names := make([]string, 0, len(req.Header)+1)
	for k, v := range req.Header {
		lk := strings.ToLower(k)
		values[lk] = strings.Join(v, ",")
		names = append(names, lk)
	}
	if _, ok := values["host"]; !ok {
		host := req.URL.Host
		if host == "" {
			host = req.Host
		}
		values["host"] = host
		names = append(names, "host")
	}
	sort.Strings(names)

	var b strings.Builder
	for _, n := range names {
		b.WriteString(n)
		b.WriteString(":")
		b.WriteString(strings.TrimSpace(values[n]))
		b.WriteString("\n")
	}
	return b.String(), strings.Join(names, ";")
}

func canonicalURI(req *http.Request) string {
	if req.URL.Path == "" {
		return "/"
	}
	return req.URL.EscapedPath()
}

// canonicalQuery builds the sorted, percent-encoded query string SigV4
// signs over. Bedrock's runtime endpoints carry no query parameters
// today; getting this right is what keeps that from becoming a signing
// bug the first time one does.
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

	var pairs []string
	for _, k := range keys {
		values := append([]string(nil), q[k]...)
		sort.Strings(values)
		for _, v := range values {
			pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(pairs, "&")
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
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
