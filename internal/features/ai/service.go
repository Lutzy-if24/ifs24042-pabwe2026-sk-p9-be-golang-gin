package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"app/internal/features/post"
)

// Galat domain AI.
var (
	ErrAIUnavailable     = errors.New("layanan AI sedang tidak tersedia")
	ErrInvalidAIResponse = errors.New("AI mengembalikan format artikel yang tidak valid")
)

// Searcher mencari referensi web untuk sebuah topik.
type Searcher interface {
	Search(ctx context.Context, query string) ([]SearchResult, error)
}

// Completer mengirim prompt ke model bahasa.
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Service mengorkestrasi Search Engine dan OpenAI untuk menghasilkan artikel.
type Service struct {
	search Searcher
	chat   Completer
}

// NewService membuat Service AI.
func NewService(search Searcher, chat Completer) *Service {
	return &Service{search: search, chat: chat}
}

var systemPrompt = `Kamu adalah penulis blog profesional berbahasa Indonesia untuk platform "Delcom Blog".
Tulis artikel yang informatif, runtut, dan menarik dalam format Markdown (gunakan heading ##, daftar, dan penekanan seperlunya).
Bila diberikan hasil penelusuran web, jadikan sebagai dasar fakta dan sebutkan rujukannya dengan sitasi [1], [2], dst. di dalam teks,
lalu tutup artikel dengan bagian "## Referensi" yang memuat daftar sumber bernomor beserta URL-nya. Jangan mengarang sumber.
Kategori harus salah satu dari: ` + strings.Join(post.Categories, ", ") + `.
Balas HANYA dengan satu objek JSON tanpa teks lain, dengan kunci:
"title" (judul), "category" (kategori), "excerpt" (ringkasan pemikat 1-2 kalimat), "content" (isi artikel Markdown).`

func buildUserPrompt(req GenerateRequest, sources []SearchResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Topik artikel: %s\n", req.Topic)
	if req.Category != "" {
		fmt.Fprintf(&b, "Kategori yang diinginkan: %s\n", req.Category)
	}
	if len(sources) > 0 {
		b.WriteString("\nHasil penelusuran web terkini sebagai konteks:\n")
		for i, s := range sources {
			fmt.Fprintf(&b, "[%d] %s (%s)\n%s\n", i+1, s.Title, s.URL, s.Snippet)
		}
	}
	return b.String()
}

// extractJSON mengambil objek JSON dari jawaban model (menoleransi pagar kode ``` dan teks pengantar).
func extractJSON(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return raw
	}
	return raw[start : end+1]
}

func pickCategory(candidates ...string) string {
	for _, c := range candidates {
		if post.IsValidCategory(c) {
			return c
		}
	}
	return post.DefaultCategory
}

type draft struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Excerpt  string `json:"excerpt"`
	Content  string `json:"content"`
}

// Generate menghasilkan draf artikel. Kegagalan Search Engine tidak menggagalkan proses:
// artikel tetap dibuat tanpa konteks web dan peringatan dikembalikan pada SearchWarning.
func (s *Service) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	res := &GenerateResponse{Sources: []SearchResult{}}

	if req.IncludeWebSearch {
		sources, err := s.search.Search(ctx, req.Topic)
		if err != nil {
			res.SearchWarning = "Penelusuran web gagal, artikel dibuat tanpa referensi web"
		} else {
			res.Sources = append(res.Sources, sources...)
			res.UsedWebSearch = len(sources) > 0
		}
	}

	raw, err := s.chat.Complete(ctx, systemPrompt, buildUserPrompt(req, res.Sources))
	if err != nil {
		return nil, ErrAIUnavailable
	}

	var d draft
	if err := json.Unmarshal([]byte(extractJSON(raw)), &d); err != nil {
		return nil, ErrInvalidAIResponse
	}
	d.Title = strings.TrimSpace(d.Title)
	if d.Title == "" || strings.TrimSpace(d.Content) == "" {
		return nil, ErrInvalidAIResponse
	}

	res.Title = d.Title
	res.Category = pickCategory(d.Category, req.Category)
	res.Excerpt = d.Excerpt
	res.Content = d.Content
	return res, nil
}
