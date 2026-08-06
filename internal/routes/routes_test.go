package routes_test

import (
	"bytes"
	"encoding/json"
	"github.com/Soumya03007/pulseboard/internal/config"
	"github.com/Soumya03007/pulseboard/internal/migrations"
	"github.com/Soumya03007/pulseboard/internal/routes"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestAuthenticationFlow(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := config.OpenDatabase(url)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("TRUNCATE users RESTART IDENTITY").Error; err != nil {
		t.Fatal(err)
	}
	router := routes.NewRouter(db, "test-secret")
	register := call(router, http.MethodPost, "/api/auth/register", map[string]string{"email": "person@example.com", "password": "password"}, "")
	if register.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", register.Code, register.Body.String())
	}
	duplicate := call(router, http.MethodPost, "/api/auth/register", map[string]string{"email": "person@example.com", "password": "password"}, "")
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate: %d", duplicate.Code)
	}
	login := call(router, http.MethodPost, "/api/auth/login", map[string]string{"email": "person@example.com", "password": "password"}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login: %d", login.Code)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &body); err != nil || body.Token == "" {
		t.Fatal("missing token")
	}
	if response := call(router, http.MethodGet, "/api/me", nil, ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: %d", response.Code)
	}
	if response := call(router, http.MethodGet, "/api/me", nil, "Bearer bad"); response.Code != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", response.Code)
	}
	expired := signedToken(t, "test-secret", time.Now().Add(-time.Minute))
	if response := call(router, http.MethodGet, "/api/me", nil, "Bearer "+expired); response.Code != http.StatusUnauthorized {
		t.Fatalf("expired token: %d", response.Code)
	}
	wrongSecret := signedToken(t, "wrong-secret", time.Now().Add(time.Hour))
	if response := call(router, http.MethodGet, "/api/me", nil, "Bearer "+wrongSecret); response.Code != http.StatusUnauthorized {
		t.Fatalf("wrong signature: %d", response.Code)
	}
	me := call(router, http.MethodGet, "/api/me", nil, "Bearer "+body.Token)
	if me.Code != http.StatusOK || bytes.Contains(me.Body.Bytes(), []byte("password_hash")) {
		t.Fatalf("profile response: %d %s", me.Code, me.Body.String())
	}
}

func signedToken(t *testing.T, secret string, expiresAt time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": 1, "exp": expiresAt.Unix()})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func call(router http.Handler, method, path string, body map[string]string, authorization string) *httptest.ResponseRecorder {
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}
