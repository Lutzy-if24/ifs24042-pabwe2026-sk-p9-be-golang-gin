package auth

import (
	"errors"
	"strings"
	"time"

	"app/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Galat domain autentikasi.
var (
	ErrUserExists         = errors.New("username atau email sudah terdaftar")
	ErrInvalidCredentials = errors.New("username atau password salah")
	ErrUserNotFound       = errors.New("pengguna tidak ditemukan")
)

// bcryptCost dapat diturunkan pada test agar cepat.
var bcryptCost = bcrypt.DefaultCost

// HashPassword menghasilkan hash bcrypt dari password.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(b), err
}

func translate(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUserNotFound
	}
	return err
}

// normalizeRole hanya menerima "admin" atau "member"; selain itu dipaksa "member".
func normalizeRole(role string) string {
	if strings.EqualFold(strings.TrimSpace(role), RoleAdmin) {
		return RoleAdmin
	}
	return RoleMember
}

// Service berisi logika bisnis autentikasi.
type Service struct {
	repo   Repository
	secret []byte
	ttl    time.Duration
	sign   func(t *jwt.Token, key []byte) (string, error)
}

// NewService membuat Service autentikasi.
func NewService(repo Repository, secret string, ttl time.Duration) *Service {
	return &Service{
		repo:   repo,
		secret: []byte(secret),
		ttl:    ttl,
		sign:   func(t *jwt.Token, key []byte) (string, error) { return t.SignedString(key) },
	}
}

// Register mendaftarkan penulis baru. Role "admin" atau "member" diterima,
// nilai lain atau kosong dipaksa menjadi "member".
func (s *Service) Register(req RegisterRequest) (*User, error) {
	exists, err := s.repo.Exists(req.Username, req.Email, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserExists
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	u := &User{Username: req.Username, Email: req.Email, Password: hash, Role: normalizeRole(req.Role)}
	if err := s.repo.Create(u); err != nil {
		return nil, err
	}
	return u, nil
}

// GenerateToken menerbitkan JWT HS256 untuk pengguna.
func (s *Service) GenerateToken(u *User) (string, error) {
	now := time.Now()
	claims := &middleware.Claims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	return s.sign(jwt.NewWithClaims(jwt.SigningMethodHS256, claims), s.secret)
}

// Login memverifikasi kredensial (username atau email) lalu menerbitkan token.
func (s *Service) Login(req LoginRequest) (string, *User, error) {
	u, err := s.repo.FindByLogin(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, ErrInvalidCredentials
		}
		return "", nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
		return "", nil, ErrInvalidCredentials
	}
	token, err := s.GenerateToken(u)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

// Profile mengambil profil pengguna.
func (s *Service) Profile(id uint) (*User, error) {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return nil, translate(err)
	}
	return u, nil
}

// UpdateProfile memperbarui profil; field kosong tidak diubah.
func (s *Service) UpdateProfile(id uint, req UpdateProfileRequest) (*User, error) {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return nil, translate(err)
	}
	if req.Username != "" {
		u.Username = req.Username
	}
	if req.Email != "" {
		u.Email = req.Email
	}
	exists, err := s.repo.Exists(u.Username, u.Email, u.ID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserExists
	}
	if req.Password != "" {
		hash, err := HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		u.Password = hash
	}
	if err := s.repo.Update(u); err != nil {
		return nil, err
	}
	return u, nil
}