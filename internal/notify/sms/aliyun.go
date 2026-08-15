// Package sms holds one Sender implementation per SMS vendor.
package sms

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // required by Aliyun's RPC signing spec, not used for secrecy
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AliyunSender sends SMS through Alibaba Cloud's dysmsapi (the vendor
// shown configured in the design mockup for 短信/邮件配置). config is
// expected to hold: access_key_id, access_key_secret, sign_name,
// template_code, and optionally template_param_key (the template
// variable the OTP code is substituted into, defaulting to "code").
type AliyunSender struct {
	httpClient *http.Client
}

func NewAliyunSender() *AliyunSender {
	return &AliyunSender{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

const aliyunEndpoint = "https://dysmsapi.aliyuncs.com/"

func (s *AliyunSender) Send(ctx context.Context, config map[string]any, recipient, message string) error {
	accessKeyID, _ := config["access_key_id"].(string)
	accessKeySecret, _ := config["access_key_secret"].(string)
	signName, _ := config["sign_name"].(string)
	templateCode, _ := config["template_code"].(string)
	paramKey, _ := config["template_param_key"].(string)
	if paramKey == "" {
		paramKey = "code"
	}
	if accessKeyID == "" || accessKeySecret == "" || signName == "" || templateCode == "" {
		return fmt.Errorf("sms: aliyun channel is missing required config")
	}

	nonce, err := randomNonce()
	if err != nil {
		return err
	}

	params := map[string]string{
		"Action":           "SendSms",
		"Version":          "2017-05-25",
		"RegionId":         "cn-hangzhou",
		"PhoneNumbers":     recipient,
		"SignName":         signName,
		"TemplateCode":     templateCode,
		"TemplateParam":    fmt.Sprintf(`{%q:%q}`, paramKey, message),
		"Format":           "JSON",
		"AccessKeyId":      accessKeyID,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   nonce,
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	params["Signature"] = signRequest(http.MethodGet, params, accessKeySecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, aliyunEndpoint+"?"+encodeQuery(params), nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sms: aliyun request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sms: aliyun returned status %d", resp.StatusCode)
	}
	return nil
}

// signRequest implements Aliyun's RPC request signing algorithm:
// HMAC-SHA1 over "METHOD&/&<canonicalized query>", keyed with
// accessKeySecret + "&".
func signRequest(method string, params map[string]string, accessKeySecret string) string {
	stringToSign := method + "&" + aliyunPercentEncode("/") + "&" + aliyunPercentEncode(encodeQuery(params))

	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// encodeQuery builds Aliyun's canonicalized, sorted query string.
func encodeQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = aliyunPercentEncode(k) + "=" + aliyunPercentEncode(params[k])
	}
	return strings.Join(parts, "&")
}

// aliyunPercentEncode applies RFC 3986 percent-encoding with the
// specific substitutions Aliyun's signing spec requires: space becomes
// %20 (not +), and ~ is left unescaped.
func aliyunPercentEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

func randomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
