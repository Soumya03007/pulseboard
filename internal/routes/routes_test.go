package routes_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Soumya03007/pulseboard/internal/config"
	"github.com/Soumya03007/pulseboard/internal/migrations"
	"github.com/Soumya03007/pulseboard/internal/routes"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
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
	resetTestDB(t, db)
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

func TestUserProfileManagement(t *testing.T) {
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
	resetTestDB(t, db)
	router := routes.NewRouter(db, "test-secret")
	token := registerAndLogin(t, router, "profile@example.com")

	profile := callAny(router, http.MethodGet, "/api/me", nil, "Bearer "+token)
	if profile.Code != http.StatusOK {
		t.Fatalf("get profile: %d %s", profile.Code, profile.Body.String())
	}
	var payload struct {
		DisplayName   string `json:"display_name"`
		StatusMessage string `json:"status_message"`
	}
	if err := json.Unmarshal(profile.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.DisplayName != "" || payload.StatusMessage != "" {
		t.Fatalf("default profile: %+v", payload)
	}

	update := callAny(router, http.MethodPatch, "/api/me", map[string]string{"display_name": "Ada", "status_message": "Planning v1.2"}, "Bearer "+token)
	if update.Code != http.StatusOK {
		t.Fatalf("update profile: %d %s", update.Code, update.Body.String())
	}
	var updated struct {
		DisplayName   string `json:"display_name"`
		StatusMessage string `json:"status_message"`
	}
	if err := json.Unmarshal(update.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName != "Ada" || updated.StatusMessage != "Planning v1.2" {
		t.Fatalf("updated profile: %s", update.Body.String())
	}

	invalid := callAny(router, http.MethodPatch, "/api/me", map[string]string{"display_name": "   "}, "Bearer "+token)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("blank display name: %d %s", invalid.Code, invalid.Body.String())
	}

	deleteResponse := callAny(router, http.MethodDelete, "/api/me", nil, "Bearer "+token)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete profile: %d %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if response := callAny(router, http.MethodGet, "/api/me", nil, "Bearer "+token); response.Code != http.StatusUnauthorized {
		t.Fatalf("profile after delete: %d", response.Code)
	}
}

func TestUserStateAndActivityFlow(t *testing.T) {
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
	resetTestDB(t, db)
	router := routes.NewRouter(db, "test-secret")
	token := registerAndLogin(t, router, "state@example.com")

	stateUpdate := callAny(router, http.MethodPatch, "/api/me", map[string]string{"presence": "away", "availability": "in_meeting"}, "Bearer "+token)
	if stateUpdate.Code != http.StatusOK {
		t.Fatalf("state update: %d %s", stateUpdate.Code, stateUpdate.Body.String())
	}
	var profile struct {
		Presence     string `json:"presence"`
		Availability string `json:"availability"`
	}
	if err := json.Unmarshal(stateUpdate.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	if profile.Presence != "away" || profile.Availability != "in_meeting" {
		t.Fatalf("state response: %s", stateUpdate.Body.String())
	}

	createActivity := callAny(router, http.MethodPost, "/api/me/activities", map[string]string{"title": "Implementing presence flow"}, "Bearer "+token)
	if createActivity.Code != http.StatusCreated {
		t.Fatalf("create activity: %d %s", createActivity.Code, createActivity.Body.String())
	}
	var activity struct {
		ID     uint   `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(createActivity.Body.Bytes(), &activity); err != nil {
		t.Fatal(err)
	}
	if activity.ID == 0 || activity.Title != "Implementing presence flow" || activity.Status != "active" {
		t.Fatalf("activity response: %s", createActivity.Body.String())
	}

	list := callAny(router, http.MethodGet, "/api/me/activities", nil, "Bearer "+token)
	if list.Code != http.StatusOK {
		t.Fatalf("list activities: %d %s", list.Code, list.Body.String())
	}
	var activities []struct {
		ID     uint   `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &activities); err != nil {
		t.Fatal(err)
	}
	if len(activities) != 1 || activities[0].ID != activity.ID || activities[0].Status != "active" {
		t.Fatalf("activity list: %s", list.Body.String())
	}

	complete := callAny(router, http.MethodPost, "/api/me/activities/complete", nil, "Bearer "+token)
	if complete.Code != http.StatusOK {
		t.Fatalf("complete activity: %d %s", complete.Code, complete.Body.String())
	}
	var completed struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(complete.Body.Bytes(), &completed); err != nil {
		t.Fatal(err)
	}
	if completed.Status != "completed" {
		t.Fatalf("completed activity: %s", complete.Body.String())
	}

	me := callAny(router, http.MethodGet, "/api/me", nil, "Bearer "+token)
	if me.Code != http.StatusOK {
		t.Fatalf("me after complete: %d %s", me.Code, me.Body.String())
	}
	var latestProfile struct {
		CurrentActivity *struct {
			ID uint `json:"id"`
		} `json:"current_activity"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &latestProfile); err != nil {
		t.Fatal(err)
	}
	if latestProfile.CurrentActivity != nil {
		t.Fatalf("current activity should be cleared: %s", me.Body.String())
	}
}

func TestBoardsFlow(t *testing.T) {
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
	resetTestDB(t, db)
	router := routes.NewRouter(db, "test-secret")
	firstToken := registerAndLogin(t, router, "first@example.com")
	secondToken := registerAndLogin(t, router, "second@example.com")

	if response := callAny(router, http.MethodGet, "/api/boards", nil, ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("boards without token: %d", response.Code)
	}
	if response := callAny(router, http.MethodPost, "/api/boards", map[string]string{"title": "   "}, "Bearer "+firstToken); response.Code != http.StatusBadRequest {
		t.Fatalf("blank title: %d", response.Code)
	}

	create := callAny(router, http.MethodPost, "/api/boards", map[string]string{"title": "Launch", "description": "v1.1 board"}, "Bearer "+firstToken)
	if create.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", create.Code, create.Body.String())
	}
	var board struct {
		ID          uint    `json:"id"`
		OwnerID     uint    `json:"owner_id"`
		Title       string  `json:"title"`
		Description *string `json:"description"`
		DeletedAt   *string `json:"deleted_at"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &board); err != nil {
		t.Fatal(err)
	}
	if board.ID == 0 || board.OwnerID == 0 || board.Title != "Launch" || board.Description == nil || *board.Description != "v1.1 board" || board.DeletedAt != nil {
		t.Fatalf("created board response: %s", create.Body.String())
	}

	if response := callAny(router, http.MethodGet, fmt.Sprintf("/api/boards/%d", board.ID), nil, "Bearer "+secondToken); response.Code != http.StatusNotFound {
		t.Fatalf("other user get: %d", response.Code)
	}
	if response := callAny(router, http.MethodPatch, fmt.Sprintf("/api/boards/%d", board.ID), map[string]string{"title": "Nope"}, "Bearer "+secondToken); response.Code != http.StatusNotFound {
		t.Fatalf("other user update: %d", response.Code)
	}
	if response := callAny(router, http.MethodDelete, fmt.Sprintf("/api/boards/%d", board.ID), nil, "Bearer "+secondToken); response.Code != http.StatusNotFound {
		t.Fatalf("other user delete: %d", response.Code)
	}

	update := callAny(router, http.MethodPatch, fmt.Sprintf("/api/boards/%d", board.ID), map[string]string{"title": "Updated", "description": ""}, "Bearer "+firstToken)
	if update.Code != http.StatusOK {
		t.Fatalf("update: %d %s", update.Code, update.Body.String())
	}
	var updated struct {
		Title       string  `json:"title"`
		Description *string `json:"description"`
	}
	if err := json.Unmarshal(update.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated" || updated.Description != nil {
		t.Fatalf("updated board response: %s", update.Body.String())
	}

	list := callAny(router, http.MethodGet, "/api/boards", nil, "Bearer "+firstToken)
	if list.Code != http.StatusOK {
		t.Fatalf("list: %d", list.Code)
	}
	var boards []struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &boards); err != nil {
		t.Fatal(err)
	}
	if len(boards) != 1 || boards[0].ID != board.ID {
		t.Fatalf("list response: %s", list.Body.String())
	}

	remove := callAny(router, http.MethodDelete, fmt.Sprintf("/api/boards/%d", board.ID), nil, "Bearer "+firstToken)
	if remove.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", remove.Code)
	}
	if response := callAny(router, http.MethodGet, fmt.Sprintf("/api/boards/%d", board.ID), nil, "Bearer "+firstToken); response.Code != http.StatusNotFound {
		t.Fatalf("get deleted: %d", response.Code)
	}
	if response := callAny(router, http.MethodDelete, fmt.Sprintf("/api/boards/%d", board.ID), nil, "Bearer "+firstToken); response.Code != http.StatusNotFound {
		t.Fatalf("delete deleted: %d", response.Code)
	}
	empty := callAny(router, http.MethodGet, "/api/boards", nil, "Bearer "+firstToken)
	if empty.Code != http.StatusOK || empty.Body.String() != "[]" {
		t.Fatalf("empty list: %d %s", empty.Code, empty.Body.String())
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

func resetTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("TRUNCATE boards, users, activities RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatal(err)
	}
}
func call(router http.Handler, method, path string, body map[string]string, authorization string) *httptest.ResponseRecorder {
	return callAny(router, method, path, body, authorization)
}

func registerAndLogin(t *testing.T, router http.Handler, email string) string {
	t.Helper()
	register := call(router, http.MethodPost, "/api/auth/register", map[string]string{"email": email, "password": "password"}, "")
	if register.Code != http.StatusCreated {
		t.Fatalf("register %s: %d %s", email, register.Code, register.Body.String())
	}
	login := call(router, http.MethodPost, "/api/auth/login", map[string]string{"email": email, "password": "password"}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", email, login.Code, login.Body.String())
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &body); err != nil || body.Token == "" {
		t.Fatalf("missing token for %s", email)
	}
	return body.Token
}

func callAny(router http.Handler, method, path string, body interface{}, authorization string) *httptest.ResponseRecorder {
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
