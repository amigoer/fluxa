package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// FeishuAdapter implements Adapter against the Feishu (Lark) Open
// Platform OAuth flow: exchange the app's own credentials for an
// app_access_token, then exchange the user's authorization code for
// their identity using that token.
type FeishuAdapter struct {
	httpClient *http.Client
}

func NewFeishuAdapter() *FeishuAdapter {
	return &FeishuAdapter{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

const feishuAPIBase = "https://open.feishu.cn/open-apis"

func (a *FeishuAdapter) ExchangeCode(ctx context.Context, appID, appSecret, code string) (UserInfo, error) {
	appToken, err := a.appAccessToken(ctx, appID, appSecret)
	if err != nil {
		return UserInfo{}, fmt.Errorf("identity: feishu app access token: %w", err)
	}

	user, err := a.userAccessToken(ctx, appToken, code)
	if err != nil {
		return UserInfo{}, fmt.Errorf("identity: feishu user access token: %w", err)
	}

	return user, nil
}

func (a *FeishuAdapter) appAccessToken(ctx context.Context, appID, appSecret string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		feishuAPIBase+"/auth/v3/app_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	var out struct {
		Code           int    `json:"code"`
		Msg            string `json:"msg"`
		AppAccessToken string `json:"app_access_token"`
	}
	if err := a.doJSON(req, &out); err != nil {
		return "", err
	}
	if out.Code != 0 {
		return "", fmt.Errorf("feishu error %d: %s", out.Code, out.Msg)
	}
	return out.AppAccessToken, nil
}

func (a *FeishuAdapter) userAccessToken(ctx context.Context, appAccessToken, code string) (UserInfo, error) {
	body, _ := json.Marshal(map[string]string{
		"grant_type": "authorization_code",
		"code":       code,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		feishuAPIBase+"/authen/v1/access_token", bytes.NewReader(body))
	if err != nil {
		return UserInfo{}, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+appAccessToken)

	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			OpenID string `json:"open_id"`
			Name   string `json:"name"`
			Email  string `json:"email"`
			// Feishu returns the work address separately from the personal
			// one, and it is the address an admin recognises a colleague by.
			EnterpriseEmail string `json:"enterprise_email"`
			// Four sizes come back; the 640px one is the largest, and the
			// console renders avatars at 22-28px on ordinary displays and
			// much larger on the identity menu, so it is the one that does
			// not go soft anywhere.
			AvatarBig string `json:"avatar_big"`
			AvatarURL string `json:"avatar_url"`
		} `json:"data"`
	}
	if err := a.doJSON(req, &out); err != nil {
		return UserInfo{}, err
	}
	if out.Code != 0 {
		return UserInfo{}, fmt.Errorf("feishu error %d: %s", out.Code, out.Msg)
	}

	email := out.Data.EnterpriseEmail
	if email == "" {
		email = out.Data.Email
	}
	avatar := out.Data.AvatarBig
	if avatar == "" {
		avatar = out.Data.AvatarURL
	}

	return UserInfo{
		ExternalUserID: out.Data.OpenID,
		Name:           out.Data.Name,
		Email:          email,
		AvatarURL:      avatar,
	}, nil
}

func (a *FeishuAdapter) doJSON(req *http.Request, out any) error {
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
