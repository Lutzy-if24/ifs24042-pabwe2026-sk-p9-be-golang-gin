package post

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Galat domain artikel.
var (
	ErrPostNotFound    = errors.New("artikel tidak ditemukan")
	ErrPostForbidden   = errors.New("anda tidak memiliki akses ke artikel ini")
	ErrInvalidCategory = errors.New("kategori tidak valid")
)

func translate(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPostNotFound
	}
	return err
}

// Service berisi logika bisnis artikel.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService membuat Service artikel.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func normalizeCategory(c string) (string, error) {
	c = strings.TrimSpace(c)
	if c == "" {
		return DefaultCategory, nil
	}
	if !IsValidCategory(c) {
		return "", ErrInvalidCategory
	}
	return c, nil
}

// uniqueSlug membuat slug dari judul; bila sudah dipakai ditambah sufiks -2, -3, dst.
func (s *Service) uniqueSlug(title string) (string, error) {
	base := Slugify(title)
	slug := base
	for i := 2; ; i++ {
		exists, err := s.repo.SlugExists(slug)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// setPublished mengatur status terbit sekaligus timestamp published_at.
func (s *Service) setPublished(p *Post, publish bool) {
	if publish {
		t := s.now()
		p.PublishedAt = &t
	} else {
		p.PublishedAt = nil
	}
	p.IsPublished = publish
}

// CreatePost membuat artikel baru dengan slug unik.
func (s *Service) CreatePost(userID uint, req CreatePostRequest) (*Post, error) {
	category, err := normalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}
	slug, err := s.uniqueSlug(req.Title)
	if err != nil {
		return nil, err
	}
	p := &Post{
		UserID:         userID,
		Title:          strings.TrimSpace(req.Title),
		Slug:           slug,
		Category:       category,
		Excerpt:        req.Excerpt,
		Content:        req.Content,
		CoverImagePath: req.CoverImagePath,
	}
	s.setPublished(p, req.IsPublished)
	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdatePost memperbarui artikel milik penulis.
func (s *Service) UpdatePost(userID, id uint, req UpdatePostRequest) (*Post, error) {
	p, err := s.repo.FindOwned(id, userID)
	if err != nil {
		// Jika tidak ditemukan di FindOwned, cek apakah artikelnya ada tapi milik orang lain
		if errors.Is(err, gorm.ErrRecordNotFound) {
			exists, _ := s.repo.FindByID(id)
			if exists != nil {
				return nil, ErrPostForbidden // Mengembalikan 403
			}
		}
		return nil, translate(err) // Mengembalikan 404
	}
	category, err := normalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}
	p.Title = strings.TrimSpace(req.Title)
	p.Category = category
	p.Excerpt = req.Excerpt
	p.Content = req.Content
	p.CoverImagePath = req.CoverImagePath
	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

// TogglePublish membalik status Draf <-> Terbit beserta published_at.
func (s *Service) TogglePublish(userID, id uint) (*Post, error) {
	p, err := s.repo.FindOwned(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			exists, _ := s.repo.FindByID(id)
			if exists != nil {
				return nil, ErrPostForbidden // Mengembalikan 403
			}
		}
		return nil, translate(err)
	}
	s.setPublished(p, !p.IsPublished)
	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

// DeletePost menghapus artikel milik penulis.
func (s *Service) DeletePost(userID, id uint) error {
	if _, err := s.repo.FindOwned(id, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			exists, _ := s.repo.FindByID(id)
			if exists != nil {
				return ErrPostForbidden // Mengembalikan 403
			}
		}
		return translate(err)
	}
	return s.repo.Delete(id)
}

// GetOwned mengambil artikel milik penulis (untuk diedit).
func (s *Service) GetOwned(userID, id uint) (*Post, error) {
	p, err := s.repo.FindOwned(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			exists, _ := s.repo.FindByID(id)
			if exists != nil {
				return nil, ErrPostForbidden // Mengembalikan 403
			}
		}
		return nil, translate(err)
	}
	return p, nil
}

// ListMine mengembalikan seluruh artikel milik penulis (terbit dan draf).
func (s *Service) ListMine(userID uint) ([]Post, error) { return s.repo.ListByUser(userID) }

// Stats menghitung ringkasan 3 kartu statistik penulis.
func (s *Service) Stats(userID uint) (*StatsResponse, error) {
	total, published, err := s.repo.Stats(userID)
	if err != nil {
		return nil, err
	}
	return &StatsResponse{
		TotalArticles: total,
		TotalSelesai:  published,
		BelumSelesai:  total - published,
	}, nil
}

// PublicList mengembalikan artikel terbit dengan pencarian kata kunci dan filter kategori.
func (s *Service) PublicList(q, category string) ([]Post, error) {
	return s.repo.ListPublished(q, strings.TrimSpace(category))
}

// PublicDetail mengambil artikel terbit berdasarkan slug.
func (s *Service) PublicDetail(slug string) (*Post, error) {
	p, err := s.repo.FindPublishedBySlug(slug)
	if err != nil {
		return nil, translate(err)
	}
	return p, nil
}

// AdminUsers mengembalikan seluruh pengguna beserta total artikel.
func (s *Service) AdminUsers() ([]AdminUserResponse, error) { return s.repo.AdminUsers() }

// AdminPosts mengembalikan arsip seluruh artikel dari semua penulis.
func (s *Service) AdminPosts() ([]Post, error) { return s.repo.ListAll() }

// AdminUserPosts mengembalikan seluruh artikel milik pengguna tertentu.
func (s *Service) AdminUserPosts(userID uint) ([]Post, error) { return s.repo.ListByUser(userID) }