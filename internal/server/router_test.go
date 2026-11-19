package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/internal/config"
	"app/internal/database"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

type env struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setup(t *testing.T) *gin.Engine {
	t.Helper()

	searchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"results":[{"title":"Sumber Web","url":"https://contoh.id","snippet":"fakta terbaru"}]}`)
	}))
	t.Cleanup(searchSrv.Close)

	chatSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := `{"title":"Artikel AI","category":"Teknologi","excerpt":"ringkas","content":"isi [1]"}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": content}}},
		})
	}))
	t.Cleanup(chatSrv.Close)

	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	cfg := &config.Config{
		AppKey: "kunci-rahasia", AdminUsername: "admin", AdminPassword: "admin",
		SearchEngineEndpoint: searchSrv.URL, OpenAIEndpoint: chatSrv.URL, OpenAIKey: "k", OpenAIModel: "m",
	}
	require.NoError(t, database.Migrate(db))
	require.NoError(t, database.Seed(db, cfg))
	return NewRouter(cfg, db, http.DefaultClient)
}

func call(t *testing.T, r http.Handler, method, path string, body any, token string) (int, env) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var e env
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &e), w.Body.String())
	return w.Code, e
}

func login(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()
	code, e := call(t, r, "POST", "/api/auth/login", map[string]string{"username": username, "password": password}, "")
	require.Equal(t, http.StatusOK, code, e.Message)
	var data struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(e.Data, &data))
	return data.Token
}

func TestEndToEndBlogFlow(t *testing.T) {
	r := setup(t)

	// akun seeder dapat login; penulis baru dapat mendaftar
	admin := login(t, r, "admin", "admin")
	member := login(t, r, "member", "member")
	code, _ := call(t, r, "POST", "/api/auth/register",
		map[string]string{"username": "penulis", "email": "penulis@mail.id", "password": "rahasia1"}, "")
	assert.Equal(t, http.StatusCreated, code)
	penulis := login(t, r, "penulis", "rahasia1")

	// penulis membuat artikel, menerbitkan, lalu tampil di publik
	code, e := call(t, r, "POST", "/api/posts",
		map[string]any{"title": "Belajar Golang Gin", "category": "Teknologi", "content": "isi artikel"}, penulis)
	require.Equal(t, http.StatusCreated, code)
	var created struct {
		ID   uint   `json:"id"`
		Slug string `json:"slug"`
	}
	require.NoError(t, json.Unmarshal(e.Data, &created))
	assert.Equal(t, "belajar-golang-gin", created.Slug)

	code, _ = call(t, r, "PATCH", "/api/posts/1/toggle-publish", nil, penulis)
	assert.Equal(t, http.StatusOK, code)

	code, e = call(t, r, "GET", "/api/public/posts?q=golang", nil, "")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(e.Data), "belajar-golang-gin")
	code, e = call(t, r, "GET", "/api/public/posts/belajar-golang-gin", nil, "")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(e.Data), `"username":"penulis"`)

	code, e = call(t, r, "GET", "/api/posts/stats", nil, penulis)
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(e.Data), `"total_selesai":1`)

	// AI generator terintegrasi Search Engine + OpenAI (keduanya mock)
	code, e = call(t, r, "POST", "/api/posts/ai-generate",
		map[string]any{"topic": "Masa depan AI", "include_web_search": true}, member)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(e.Data), "Artikel AI")
	assert.Contains(t, string(e.Data), "Sumber Web")
	assert.Contains(t, string(e.Data), `"used_web_search":true`)

	// kontrol akses
	code, _ = call(t, r, "GET", "/api/posts", nil, "")
	assert.Equal(t, http.StatusUnauthorized, code)
	code, _ = call(t, r, "GET", "/api/admin/users", nil, member)
	assert.Equal(t, http.StatusForbidden, code)
	code, e = call(t, r, "GET", "/api/admin/users", nil, admin)
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(e.Data), `"username":"penulis"`)
	code, e = call(t, r, "GET", "/api/admin/posts", nil, admin)
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(e.Data), "belajar-golang-gin")
}

func TestNotFoundAndDocs(t *testing.T) {
	r := setup(t)

	code, e := call(t, r, "GET", "/tidak/ada", nil, "")
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "fail", e.Status)
	assert.Equal(t, "Endpoint not found", e.Message)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/swagger/index.html", w.Header().Get("Location"))

	for _, path := range []string{"/swagger/index.html", "/swagger/doc.json"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		assert.Equal(t, http.StatusOK, w.Code, path)
	}
}

func TestAPIJSONSchemeDetection(t *testing.T) {
	r := setup(t)

	get := func(mutate func(*http.Request)) map[string]any {
		req := httptest.NewRequest("GET", "http://api.delcom.test/api.json", nil)
		mutate(req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var doc map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
		return doc
	}

	doc := get(func(*http.Request) {})
	assert.Equal(t, "api.delcom.test", doc["host"])
	assert.Equal(t, []any{"http"}, doc["schemes"])

	doc = get(func(req *http.Request) { req.Header.Set("X-Forwarded-Proto", "https") })
	assert.Equal(t, []any{"https"}, doc["schemes"])

	doc = get(func(req *http.Request) { req.TLS = &tls.ConnectionState{} })
	assert.Equal(t, []any{"https"}, doc["schemes"])
}
