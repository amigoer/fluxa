package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/amigoer/fluxa/internal/router"
	"github.com/amigoer/fluxa/internal/store"
)

func newAdminFixture(t *testing.T) (*http.ServeMux, *store.Store, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fluxa.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(upstream.Close)

	// Seed one provider so the router has something to resolve.
	if err := st.UpsertProvider(t.Context(), store.Provider{
		Name: "openai", Kind: "openai", APIKey: "sk", BaseURL: upstream.URL, Enabled: true,
	}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	// Seed an admin user and mint a session token so the test can call
	// the protected endpoints. The token replaces the old static
	// master key as the value of the Authorization Bearer header.
	user, err := st.CreateAdminUser(t.Context(), "admin", "admin1234")
	if err != nil {
		t.Fatalf("seed admin user: %v", err)
	}
	sess, err := st.CreateSession(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	r := router.New()
	provs, routes, _ := st.LoadRouterInputs(t.Context())
	if err := r.Reload(provs, routes); err != nil {
		t.Fatalf("reload: %v", err)
	}

	mux := http.NewServeMux()
	NewAdmin(r, st, nil, nil).Routes(mux)
	return mux, st, sess.Token
}

func doAdmin(t *testing.T, mux *http.ServeMux, method, path, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAdmin_AuthRequired(t *testing.T) {
	mux, _, _ := newAdminFixture(t)
	rec := doAdmin(t, mux, "GET", "/admin/providers", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("missing key should 401, got %d", rec.Code)
	}
	rec = doAdmin(t, mux, "GET", "/admin/providers", "wrong", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong key should 401, got %d", rec.Code)
	}
}

func TestAdmin_ProviderLifecycle(t *testing.T) {
	mux, _, key := newAdminFixture(t)

	// List returns the seeded provider.
	rec := doAdmin(t, mux, "GET", "/admin/providers", key, nil)
	if rec.Code != 200 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	var list struct {
		Data []providerDTO `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Data) != 1 || list.Data[0].Name != "openai" {
		t.Fatalf("seed missing: %+v", list)
	}

	// Create a new provider.
	rec = doAdmin(t, mux, "POST", "/admin/providers", key, providerDTO{
		Name: "deepseek", Kind: "deepseek", APIKey: "sk-d",
	})
	if rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}

	// Get by name.
	rec = doAdmin(t, mux, "GET", "/admin/providers/deepseek", key, nil)
	if rec.Code != 200 {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	var dto providerDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &dto)
	if dto.APIKey != "sk-d" {
		t.Errorf("unexpected dto: %+v", dto)
	}

	// Update via PUT using URL-provided name.
	rec = doAdmin(t, mux, "PUT", "/admin/providers/deepseek", key, providerDTO{
		Kind: "deepseek", APIKey: "sk-rotated",
	})
	if rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &dto)
	if dto.APIKey != "sk-rotated" {
		t.Errorf("update not applied: %+v", dto)
	}

	// Delete.
	rec = doAdmin(t, mux, "DELETE", "/admin/providers/deepseek", key, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", rec.Code, rec.Body)
	}
	rec = doAdmin(t, mux, "GET", "/admin/providers/deepseek", key, nil)
	if rec.Code != 404 {
		t.Errorf("get after delete: %d", rec.Code)
	}
}

func TestAdmin_RouteCreateReloadsRouter(t *testing.T) {
	mux, _, key := newAdminFixture(t)

	rec := doAdmin(t, mux, "POST", "/admin/routes", key, routeDTO{
		Model: "gpt-4o", Provider: "openai",
	})
	if rec.Code != 200 {
		t.Fatalf("create route: %d %s", rec.Code, rec.Body)
	}

	// Route referencing an unknown provider must fail reload and surface a
	// 400 back to the operator.
	rec = doAdmin(t, mux, "POST", "/admin/routes", key, routeDTO{
		Model: "ghost", Provider: "nope",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on bogus provider, got %d %s", rec.Code, rec.Body)
	}
}

func TestAdmin_LoginFlow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fluxa.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	if _, err := st.CreateAdminUser(t.Context(), "alice", "wonderland"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	mux := http.NewServeMux()
	NewAdmin(router.New(), st, nil, nil).Routes(mux)

	// Wrong password → 401.
	rec := doAdmin(t, mux, "POST", "/admin/auth/login", "", map[string]string{
		"username": "alice", "password": "nope",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password should 401, got %d", rec.Code)
	}

	// Correct credentials → token + user.
	rec = doAdmin(t, mux, "POST", "/admin/auth/login", "", map[string]string{
		"username": "alice", "password": "wonderland",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Token string `json:"token"`
		User  struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Token == "" || resp.User.Username != "alice" {
		t.Fatalf("login response missing fields: %+v", resp)
	}

	// Token grants access to /admin/auth/me.
	rec = doAdmin(t, mux, "GET", "/admin/auth/me", resp.Token, nil)
	if rec.Code != 200 {
		t.Errorf("me: %d %s", rec.Code, rec.Body)
	}

	// Logout invalidates the token.
	rec = doAdmin(t, mux, "POST", "/admin/auth/logout", resp.Token, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("logout: %d", rec.Code)
	}
	rec = doAdmin(t, mux, "GET", "/admin/auth/me", resp.Token, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked token should 401, got %d", rec.Code)
	}
}
