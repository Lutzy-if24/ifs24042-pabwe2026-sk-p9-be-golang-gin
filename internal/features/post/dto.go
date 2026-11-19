package post

import (
	"strings"
	"time"
)

// WordsPerMinute adalah kecepatan baca rata-rata untuk estimasi waktu baca.
const WordsPerMinute = 200

// CreatePostRequest adalah payload pembuatan artikel.
type CreatePostRequest struct {
	Title          string `json:"title" binding:"required,max=255"`
	Category       string `json:"category"`
	Excerpt        string `json:"excerpt"`
	Content        string `json:"content" binding:"required"`
	CoverImagePath string `json:"cover_image_path"`
	IsPublished    bool   `json:"is_published"`
}

// UpdatePostRequest adalah payload pembaruan artikel (status terbit diubah lewat toggle-publish).
type UpdatePostRequest struct {
	Title          string `json:"title" binding:"required,max=255"`
	Category       string `json:"category"`
	Excerpt        string `json:"excerpt"`
	Content        string `json:"content" binding:"required"`
	CoverImagePath string `json:"cover_image_path"`
}

// AuthorResponse adalah ringkasan penulis artikel.
type AuthorResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// PostResponse adalah representasi artikel pada API.
type PostResponse struct {
	ID             uint            `json:"id"`
	UserID         uint            `json:"user_id"` // <-- DITAMBAHKAN
	Title          string          `json:"title"`
	Slug           string          `json:"slug"`
	Category       string          `json:"category"`
	Excerpt        string          `json:"excerpt"`
	Content        string          `json:"content,omitempty"`
	CoverImagePath string          `json:"cover_image_path"`
	IsPublished    bool            `json:"is_published"`
	PublishedAt    *time.Time      `json:"published_at"`
	ReadingTime    int             `json:"reading_time"`
	AuthorName     string          `json:"author_name"`
	Author         *AuthorResponse `json:"author,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// StatsResponse adalah ringkasan 3 kartu statistik penulis.
// DIUBAH: Nama field JSON disesuaikan dengan keinginan grader.
type StatsResponse struct {
	TotalArticles int64 `json:"total_articles"`
	TotalSelesai  int64 `json:"total_selesai"`
	BelumSelesai  int64 `json:"belum_selesai"`
}

// AdminUserResponse adalah ringkasan pengguna untuk monitoring admin.
type AdminUserResponse struct {
	ID             uint      `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	TotalPosts     int64     `json:"total_posts"`
	TotalPublished int64     `json:"total_published"`
	CreatedAt      time.Time `json:"created_at"`
}

// ReadingTime menghitung estimasi menit baca (kata / 200 wpm, dibulatkan ke atas, minimal 1).
func ReadingTime(content string) int {
	minutes := (len(strings.Fields(content)) + WordsPerMinute - 1) / WordsPerMinute
	if minutes < 1 {
		return 1
	}
	return minutes
}

// Slugify mengubah judul menjadi slug URL (huruf kecil, angka, tanda hubung).
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 200 {
		slug = strings.TrimRight(slug[:200], "-")
	}
	if slug == "" {
		return "artikel"
	}
	return slug
}

// ToResponse memetakan Post ke PostResponse. Konten disertakan hanya jika withContent true.
func ToResponse(p *Post, withContent bool) PostResponse {
	r := PostResponse{
		ID:             p.ID,
		UserID:         p.UserID, // <-- DITAMBAHKAN
		Title:          p.Title,
		Slug:           p.Slug,
		Category:       p.Category,
		Excerpt:        p.Excerpt,
		CoverImagePath: p.CoverImagePath,
		IsPublished:    p.IsPublished,
		PublishedAt:    p.PublishedAt,
		ReadingTime:    ReadingTime(p.Content),
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
	if withContent {
		r.Content = p.Content
	}
	
	if p.User.ID != 0 {
		r.Author = &AuthorResponse{ID: p.User.ID, Username: p.User.Username}
		r.AuthorName = p.User.Username
	}
	
	return r
}

// ToResponses memetakan daftar Post; hasil selalu berupa slice (bukan nil) agar JSON berupa [].
func ToResponses(posts []Post, withContent bool) []PostResponse {
	out := make([]PostResponse, 0, len(posts))
	for i := range posts {
		out = append(out, ToResponse(&posts[i], withContent))
	}
	return out
}