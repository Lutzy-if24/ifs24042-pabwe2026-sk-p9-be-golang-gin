// Package post menangani artikel blog: CRUD, slug unik, estimasi waktu baca,
// status terbit/draf, statistik penulis dan monitoring admin.
package post

import (
	"time"

	"app/internal/features/auth"
)

// Categories adalah daftar kategori artikel yang valid.
var Categories = []string{"Teknologi", "Pendidikan", "Gaya Hidup", "Bisnis", "Opini", "Umum"}

// DefaultCategory dipakai jika kategori tidak diisi.
const DefaultCategory = "Umum"

// IsValidCategory memeriksa apakah kategori terdaftar.
func IsValidCategory(c string) bool {
	for _, v := range Categories {
		if v == c {
			return true
		}
	}
	return false
}

// Post adalah model tabel posts.
type Post struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"not null;index" json:"user_id"`
	User           auth.User  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Title          string     `gorm:"size:255;not null" json:"title"`
	Slug           string     `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	Category       string     `gorm:"size:100;not null;default:Umum" json:"category"`
	Excerpt        string     `gorm:"type:text" json:"excerpt"`
	Content        string     `gorm:"type:text" json:"content"`
	CoverImagePath string     `gorm:"type:text" json:"cover_image_path"`
	IsPublished    bool       `gorm:"not null;default:false" json:"is_published"`
	PublishedAt    *time.Time `json:"published_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
