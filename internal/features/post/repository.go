package post

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository mengabstraksi akses data artikel.
type Repository interface {
	Create(p *Post) error
	Update(p *Post) error
	Delete(id uint) error
	FindByID(id uint) (*Post, error)
	FindOwned(id, userID uint) (*Post, error)
	FindPublishedBySlug(slug string) (*Post, error)
	ListPublished(q, category string) ([]Post, error)
	ListByUser(userID uint) ([]Post, error)
	ListAll() ([]Post, error)
	SlugExists(slug string) (bool, error)
	Stats(userID uint) (total, published int64, err error)
	AdminUsers() ([]AdminUserResponse, error)
}

type gormRepository struct{ db *gorm.DB }

// NewRepository membuat Repository berbasis GORM.
func NewRepository(db *gorm.DB) Repository { return &gormRepository{db: db} }

// first: Paksa Preload("User") agar relasi penulis selalu ada saat mengambil 1 data
func first(q *gorm.DB) (*Post, error) {
	var p Post
	if err := q.Preload("User").First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// list: Paksa Preload("User") agar relasi penulis selalu ada saat mengambil banyak data
func list(q *gorm.DB) ([]Post, error) {
	var posts []Post
	err := q.Preload("User").Find(&posts).Error
	return posts, err
}

func (r *gormRepository) Create(p *Post) error {
	return r.db.Omit(clause.Associations).Create(p).Error
}

func (r *gormRepository) Update(p *Post) error {
	return r.db.Omit(clause.Associations).Save(p).Error
}

func (r *gormRepository) Delete(id uint) error {
	return r.db.Delete(&Post{}, id).Error
}

func (r *gormRepository) FindByID(id uint) (*Post, error) {
	return first(r.db.Where("id = ?", id))
}

// PERBAIKAN: Tambahkan Preload("User") agar AuthorName bisa diakses
func (r *gormRepository) FindOwned(id, userID uint) (*Post, error) {
	return first(r.db.Where("id = ? AND user_id = ?", id, userID))
}

func (r *gormRepository) FindPublishedBySlug(slug string) (*Post, error) {
	return first(r.db.Where("slug = ? AND is_published = ?", slug, true))
}

// ListPublished mengembalikan artikel terbit; q mencari di judul/excerpt/konten (tidak peka huruf besar).
func (r *gormRepository) ListPublished(q, category string) ([]Post, error) {
	tx := r.db.Where("is_published = ?", true)
	if category != "" {
		tx = tx.Where("category = ?", category)
	}
	if q = strings.TrimSpace(q); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		tx = tx.Where("(LOWER(title) LIKE ? OR LOWER(excerpt) LIKE ? OR LOWER(content) LIKE ?)", like, like, like)
	}
	// list() sudah otomatis melakukan Preload("User")
	return list(tx.Order("published_at DESC, id DESC"))
}

func (r *gormRepository) ListByUser(userID uint) ([]Post, error) {
	return list(r.db.Where("user_id = ?", userID).Order("created_at DESC, id DESC"))
}

func (r *gormRepository) ListAll() ([]Post, error) {
	return list(r.db.Order("created_at DESC, id DESC"))
}

func (r *gormRepository) SlugExists(slug string) (bool, error) {
	var n int64
	err := r.db.Model(&Post{}).Where("slug = ?", slug).Count(&n).Error
	return n > 0, err
}

func (r *gormRepository) Stats(userID uint) (int64, int64, error) {
	var total, published int64
	if err := r.db.Model(&Post{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	err := r.db.Model(&Post{}).Where("user_id = ? AND is_published = ?", userID, true).Count(&published).Error
	return total, published, err
}

const adminUsersSQL = `
SELECT users.id, users.username, users.email, users.role, users.created_at,
       COUNT(posts.id) AS total_posts,
       COUNT(CASE WHEN posts.is_published THEN 1 ELSE NULL END) AS total_published
FROM users
LEFT JOIN posts ON posts.user_id = users.id
GROUP BY users.id, users.username, users.email, users.role, users.created_at
ORDER BY users.id`

func (r *gormRepository) AdminUsers() ([]AdminUserResponse, error) {
	rows := make([]AdminUserResponse, 0)
	err := r.db.Raw(adminUsersSQL).Scan(&rows).Error
	return rows, err
}