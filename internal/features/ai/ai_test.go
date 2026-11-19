package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

var errBoom = errors.New("boom")

// ---------- Search client ----------

func searchServer(t *testing.T, handler http.HandlerFunc) *SearchClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewSearchClient(srv.URL, srv.Client())
}

func TestSearchClient(t *testing.T) {
	ctx := context.Background()

	t.Run("kunci results dan query terkirim", func(t *testing.T) {
		var gotQuery string
		c := searchServer(t, func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query().Get("q")
			_, _ = io.WriteString(w, `{"results":[{"title":"T","url":"https://a.id","snippet":"S"}]}`)
		})
		res, err := c.Search(ctx, "golang & gin")
		require.NoError(t, err)
		assert.Equal(t, "golang & gin", gotQuery)
		assert.Equal(t, []SearchResult{{Title: "T", URL: "https://a.id", Snippet: "S"}}, res)
	})

	t.Run("kunci data dan pembatasan jumlah", func(t *testing.T) {
		items := make([]SearchResult, 8)
		for i := range items {
			items[i] = SearchResult{Title: "t"}
		}
		c := searchServer(t, func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
		})
		res, err := c.Search(ctx, "x")
		require.NoError(t, err)
		assert.Len(t, res, maxSearchResults)
	})

	t.Run("status bukan 2xx", func(t *testing.T) {
		c := searchServer(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "gagal", http.StatusInternalServerError)
		})
		_, err := c.Search(ctx, "x")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("JSON rusak", func(t *testing.T) {
		c := searchServer(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "{bad") })
		_, err := c.Search(ctx, "x")
		assert.Error(t, err)
	})

	t.Run("koneksi gagal", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		client := NewSearchClient(srv.URL, srv.Client())
		srv.Close()
		_, err := client.Search(ctx, "x")
		assert.Error(t, err)
	})

	t.Run("endpoint tidak valid", func(t *testing.T) {
		_, err := NewSearchClient("://bad", http.DefaultClient).Search(ctx, "x")
		assert.Error(t, err)
	})
}

// ---------- Chat client ----------

func chatServer(t *testing.T, handler http.HandlerFunc) *ChatClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewChatClient(srv.URL+"/", "kunci", "model-x", srv.Client())
}

func TestChatClient(t *testing.T) {
	ctx := context.Background()

	t.Run("sukses dan request benar", func(t *testing.T) {
		var gotAuth, gotPath string
		var gotBody chatRequest
		c := chatServer(t, func(w http.ResponseWriter, r *http.Request) {
			gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"halo"}}]}`)
		})
		out, err := c.Complete(ctx, "sistem", "pengguna")
		require.NoError(t, err)
		assert.Equal(t, "halo", out)
		assert.Equal(t, "Bearer kunci", gotAuth)
		assert.Equal(t, "/chat/completions", gotPath)
		assert.Equal(t, "model-x", gotBody.Model)
		require.Len(t, gotBody.Messages, 2)
		assert.Equal(t, "sistem", gotBody.Messages[0].Content)
		assert.Equal(t, "pengguna", gotBody.Messages[1].Content)
	})

	t.Run("pilihan kosong", func(t *testing.T) {
		c := chatServer(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{"choices":[]}`) })
		_, err := c.Complete(ctx, "s", "u")
		assert.ErrorIs(t, err, ErrEmptyCompletion)
	})

	t.Run("status bukan 2xx", func(t *testing.T) {
		c := chatServer(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "limit", http.StatusTooManyRequests) })
		_, err := c.Complete(ctx, "s", "u")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "429")
	})

	t.Run("endpoint tidak valid", func(t *testing.T) {
		_, err := NewChatClient("://bad", "k", "m", http.DefaultClient).Complete(ctx, "s", "u")
		assert.Error(t, err)
	})
}

// ---------- Service ----------

type fakeSearch struct {
	results []SearchResult
	err     error
	called  bool
}

func (f *fakeSearch) Search(context.Context, string) ([]SearchResult, error) {
	f.called = true
	return f.results, f.err
}

type fakeChat struct {
	reply  string
	err    error
	prompt string
}

func (f *fakeChat) Complete(_ context.Context, _, user string) (string, error) {
	f.prompt = user
	return f.reply, f.err
}

const goodReply = "```json\n{\"title\":\" Judul AI \",\"category\":\"Teknologi\",\"excerpt\":\"ringkas\",\"content\":\"## Isi\\nteks [1]\"}\n```"

func TestExtractJSON(t *testing.T) {
	assert.Equal(t, `{"a":1}`, extractJSON("```json\n{\"a\":1}\n```"))
	assert.Equal(t, `{"a":1}`, extractJSON("Berikut: {\"a\":1} selesai"))
	assert.Equal(t, `{"a":1}`, extractJSON(`{"a":1}`))
	assert.Equal(t, "tanpa json", extractJSON("tanpa json"))
	assert.Equal(t, "} {", extractJSON("} {"))
}

func TestPickCategory(t *testing.T) {
	assert.Equal(t, "Bisnis", pickCategory("Bisnis", "Opini"))
	assert.Equal(t, "Opini", pickCategory("Ngawur", "Opini"))
	assert.Equal(t, "Umum", pickCategory("Ngawur", ""))
}

func TestBuildUserPrompt(t *testing.T) {
	plain := buildUserPrompt(GenerateRequest{Topic: "AI"}, nil)
	assert.Contains(t, plain, "Topik artikel: AI")
	assert.NotContains(t, plain, "Kategori yang diinginkan")
	assert.NotContains(t, plain, "penelusuran web")

	rich := buildUserPrompt(GenerateRequest{Topic: "AI", Category: "Teknologi"},
		[]SearchResult{{Title: "Judul", URL: "https://x.id", Snippet: "cuplikan"}})
	assert.Contains(t, rich, "Kategori yang diinginkan: Teknologi")
	assert.Contains(t, rich, "[1] Judul (https://x.id)")
	assert.Contains(t, rich, "cuplikan")
}

func TestGenerate(t *testing.T) {
	ctx := context.Background()
	sources := []SearchResult{{Title: "Sumber", URL: "https://x.id", Snippet: "fakta"}}

	t.Run("dengan web search", func(t *testing.T) {
		s, c := &fakeSearch{results: sources}, &fakeChat{reply: goodReply}
		res, err := NewService(s, c).Generate(ctx, GenerateRequest{Topic: "AI", IncludeWebSearch: true})
		require.NoError(t, err)
		assert.True(t, s.called)
		assert.True(t, res.UsedWebSearch)
		assert.Equal(t, sources, res.Sources)
		assert.Equal(t, "Judul AI", res.Title)
		assert.Equal(t, "Teknologi", res.Category)
		assert.Equal(t, "ringkas", res.Excerpt)
		assert.Contains(t, res.Content, "## Isi")
		assert.Contains(t, c.prompt, "fakta")
		assert.Empty(t, res.SearchWarning)
	})

	t.Run("web search tidak menemukan apa pun", func(t *testing.T) {
		res, err := NewService(&fakeSearch{}, &fakeChat{reply: goodReply}).Generate(ctx, GenerateRequest{Topic: "AI", IncludeWebSearch: true})
		require.NoError(t, err)
		assert.False(t, res.UsedWebSearch)
		assert.NotNil(t, res.Sources)
	})

	t.Run("tanpa web search", func(t *testing.T) {
		s := &fakeSearch{results: sources}
		res, err := NewService(s, &fakeChat{reply: goodReply}).Generate(ctx, GenerateRequest{Topic: "AI"})
		require.NoError(t, err)
		assert.False(t, s.called)
		assert.False(t, res.UsedWebSearch)
		assert.Empty(t, res.Sources)
	})

	t.Run("search gagal -> tetap dibuat dengan peringatan", func(t *testing.T) {
		res, err := NewService(&fakeSearch{err: errBoom}, &fakeChat{reply: goodReply}).Generate(ctx, GenerateRequest{Topic: "AI", IncludeWebSearch: true})
		require.NoError(t, err)
		assert.NotEmpty(t, res.SearchWarning)
		assert.False(t, res.UsedWebSearch)
	})

	t.Run("kategori AI tidak valid memakai kategori permintaan", func(t *testing.T) {
		reply := `{"title":"J","category":"Ngawur","content":"isi"}`
		res, err := NewService(&fakeSearch{}, &fakeChat{reply: reply}).Generate(ctx, GenerateRequest{Topic: "AI", Category: "Opini"})
		require.NoError(t, err)
		assert.Equal(t, "Opini", res.Category)
	})

	t.Run("OpenAI gagal", func(t *testing.T) {
		_, err := NewService(&fakeSearch{}, &fakeChat{err: errBoom}).Generate(ctx, GenerateRequest{Topic: "AI"})
		assert.ErrorIs(t, err, ErrAIUnavailable)
	})

	t.Run("format jawaban rusak", func(t *testing.T) {
		_, err := NewService(&fakeSearch{}, &fakeChat{reply: "bukan json"}).Generate(ctx, GenerateRequest{Topic: "AI"})
		assert.ErrorIs(t, err, ErrInvalidAIResponse)
	})

	t.Run("judul atau konten kosong", func(t *testing.T) {
		for _, reply := range []string{
			`{"title":"  ","content":"isi"}`,
			`{"title":"J","content":"   "}`,
		} {
			_, err := NewService(&fakeSearch{}, &fakeChat{reply: reply}).Generate(ctx, GenerateRequest{Topic: "AI"})
			assert.ErrorIs(t, err, ErrInvalidAIResponse, reply)
		}
	})
}

// ---------- Handler ----------

func newRouter(svc *Service) *gin.Engine {
	r := gin.New()
	noop := func(c *gin.Context) { c.Next() }
	NewHandler(svc).RegisterRoutes(r.Group("/api"), noop, noop)
	return r
}

func postJSON(r http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/posts/ai-generate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandler(t *testing.T) {
	ok := newRouter(NewService(&fakeSearch{results: []SearchResult{{Title: "S"}}}, &fakeChat{reply: goodReply}))

	w := postJSON(ok, `{"topic":"Masa depan AI","include_web_search":true}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"used_web_search":true`)
	assert.Contains(t, w.Body.String(), "Judul AI")

	assert.Equal(t, http.StatusBadRequest, postJSON(ok, `{"topic":"ab"}`).Code)
	assert.Equal(t, http.StatusBadRequest, postJSON(ok, `{bad`).Code)

	broken := newRouter(NewService(&fakeSearch{}, &fakeChat{err: errBoom}))
	w = postJSON(broken, `{"topic":"Masa depan AI"}`)
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.True(t, strings.Contains(w.Body.String(), `"status":"error"`))
}
