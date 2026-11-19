package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var errBoom = errors.New("boom")

func init() {
	gin.SetMode(gin.TestMode)
	bcryptCost = bcrypt.MinCost
}

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}))
	return db
}

func closeDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

// failRepo membungkus Repository nyata dan gagal pada metode tertentu.
type failRepo struct {
	Repository
	failOn string
}

func (f failRepo) Create(u *User) error {
	if f.failOn == "Create" {
		return errBoom
	}
	return f.Repository.Create(u)
}

func (f failRepo) Update(u *User) error {
	if f.failOn == "Update" {
		return errBoom
	}
	return f.Repository.Update(u)
}

func (f failRepo) FindByID(id uint) (*User, error) {
	if f.failOn == "FindByID" {
		return nil, errBoom
	}
	return f.Repository.FindByID(id)
}

func (f failRepo) FindByLogin(l string) (*User, error) {
	if f.failOn == "FindByLogin" {
		return nil, errBoom
	}
	return f.Repository.FindByLogin(l)
}

func (f failRepo) Exists(u, e string, id uint) (bool, error) {
	if f.failOn == "Exists" {
		return false, errBoom
	}
	return f.Repository.Exists(u, e, id)
}

func newService(t *testing.T) (*Service, Repository, *gorm.DB) {
	t.Helper()
	db := newDB(t)
	repo := NewRepository(db)
	return NewService(repo, "secret", time.Hour), repo, db
}

func failing(repo Repository, on string) *Service {
	return NewService(failRepo{Repository: repo, failOn: on}, "secret", time.Hour)
}

func registerBudi(t *testing.T, svc *Service) *User {
	t.Helper()
	u, err := svc.Register(RegisterRequest{Username: "budi", Email: "budi@mail.id", Password: "rahasia1"})
	require.NoError(t, err)
	return u
}

// ---------- Repository ----------

func TestRepository(t *testing.T) {
	db := newDB(t)
	repo := NewRepository(db)

	u := &User{Username: "budi", Email: "b@x.id", Password: "h", Role: RoleMember}
	require.NoError(t, repo.Create(u))
	assert.NotZero(t, u.ID)

	got, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, "budi", got.Username)

	got, err = repo.FindByLogin("budi")
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)
	got, err = repo.FindByLogin("b@x.id")
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)

	_, err = repo.FindByID(999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	for _, tc := range []struct {
		name, username, email string
		exclude               uint
		want                  bool
	}{
		{"username sama", "budi", "z@x.id", 0, true},
		{"email sama", "zzz", "b@x.id", 0, true},
		{"dikecualikan", "budi", "b@x.id", u.ID, false},
		{"bebas", "baru", "baru@x.id", 0, false},
	} {
		exists, err := repo.Exists(tc.username, tc.email, tc.exclude)
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.want, exists, tc.name)
	}

	u.Username = "budi2"
	require.NoError(t, repo.Update(u))
	got, err = repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, "budi2", got.Username)
}

func TestRepositoryDatabaseErrors(t *testing.T) {
	db := newDB(t)
	repo := NewRepository(db)
	closeDB(t, db)

	assert.Error(t, repo.Create(&User{Username: "a", Email: "a@x.id"}))
	assert.Error(t, repo.Update(&User{ID: 1}))
	_, err := repo.FindByID(1)
	assert.Error(t, err)
	_, err = repo.FindByLogin("a")
	assert.Error(t, err)
	_, err = repo.Exists("a", "a@x.id", 0)
	assert.Error(t, err)
}

// ---------- Service ----------

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("rahasia1")
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("rahasia1")))
}

func TestRegister(t *testing.T) {
	svc, repo, _ := newService(t)

	u := registerBudi(t, svc)
	assert.Equal(t, RoleMember, u.Role)
	assert.NotEqual(t, "rahasia1", u.Password)

	_, err := svc.Register(RegisterRequest{Username: "budi", Email: "lain@mail.id", Password: "rahasia1"})
	assert.ErrorIs(t, err, ErrUserExists)
	_, err = svc.Register(RegisterRequest{Username: "lain", Email: "budi@mail.id", Password: "rahasia1"})
	assert.ErrorIs(t, err, ErrUserExists)

	_, err = failing(repo, "Exists").Register(RegisterRequest{Username: "x1", Email: "x@mail.id", Password: "rahasia1"})
	assert.ErrorIs(t, err, errBoom)

	_, err = svc.Register(RegisterRequest{Username: "panjang", Email: "p@mail.id", Password: strings.Repeat("a", 80)})
	assert.Error(t, err)

	_, err = failing(repo, "Create").Register(RegisterRequest{Username: "x2", Email: "x2@mail.id", Password: "rahasia1"})
	assert.ErrorIs(t, err, errBoom)
}

func TestLogin(t *testing.T) {
	svc, repo, _ := newService(t)
	registerBudi(t, svc)

	for _, login := range []string{"budi", "budi@mail.id"} {
		token, u, err := svc.Login(LoginRequest{Username: login, Password: "rahasia1"})
		require.NoError(t, err)
		assert.Equal(t, "budi", u.Username)

		claims := &middleware.Claims{}
		parsed, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return []byte("secret"), nil })
		require.NoError(t, err)
		assert.True(t, parsed.Valid)
		assert.Equal(t, "budi", claims.Username)
		assert.Equal(t, RoleMember, claims.Role)
		assert.Equal(t, u.ID, claims.UserID)
	}

	_, _, err := svc.Login(LoginRequest{Username: "budi", Password: "salah"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	_, _, err = svc.Login(LoginRequest{Username: "tidakada", Password: "rahasia1"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	_, _, err = failing(repo, "FindByLogin").Login(LoginRequest{Username: "budi", Password: "rahasia1"})
	assert.ErrorIs(t, err, errBoom)

	svc.sign = func(*jwt.Token, []byte) (string, error) { return "", errBoom }
	_, _, err = svc.Login(LoginRequest{Username: "budi", Password: "rahasia1"})
	assert.ErrorIs(t, err, errBoom)
}

func TestProfile(t *testing.T) {
	svc, repo, _ := newService(t)
	u := registerBudi(t, svc)

	got, err := svc.Profile(u.ID)
	require.NoError(t, err)
	assert.Equal(t, "budi", got.Username)

	_, err = svc.Profile(999)
	assert.ErrorIs(t, err, ErrUserNotFound)
	_, err = failing(repo, "FindByID").Profile(u.ID)
	assert.ErrorIs(t, err, errBoom)
}

func TestUpdateProfile(t *testing.T) {
	svc, repo, _ := newService(t)
	u := registerBudi(t, svc)
	_, err := svc.Register(RegisterRequest{Username: "ani", Email: "ani@mail.id", Password: "rahasia1"})
	require.NoError(t, err)

	// tanpa perubahan
	same, err := svc.UpdateProfile(u.ID, UpdateProfileRequest{})
	require.NoError(t, err)
	assert.Equal(t, "budi", same.Username)

	// perubahan penuh
	updated, err := svc.UpdateProfile(u.ID, UpdateProfileRequest{Username: "budi2", Email: "budi2@mail.id", Password: "baru123"})
	require.NoError(t, err)
	assert.Equal(t, "budi2", updated.Username)
	assert.Equal(t, "budi2@mail.id", updated.Email)
	_, _, err = svc.Login(LoginRequest{Username: "budi2", Password: "baru123"})
	assert.NoError(t, err)

	_, err = svc.UpdateProfile(u.ID, UpdateProfileRequest{Username: "ani"})
	assert.ErrorIs(t, err, ErrUserExists)
	_, err = svc.UpdateProfile(999, UpdateProfileRequest{})
	assert.ErrorIs(t, err, ErrUserNotFound)
	_, err = failing(repo, "FindByID").UpdateProfile(u.ID, UpdateProfileRequest{})
	assert.ErrorIs(t, err, errBoom)
	_, err = failing(repo, "Exists").UpdateProfile(u.ID, UpdateProfileRequest{})
	assert.ErrorIs(t, err, errBoom)
	_, err = svc.UpdateProfile(u.ID, UpdateProfileRequest{Password: strings.Repeat("a", 80)})
	assert.Error(t, err)
	_, err = failing(repo, "Update").UpdateProfile(u.ID, UpdateProfileRequest{})
	assert.ErrorIs(t, err, errBoom)
}

// ---------- Handler ----------

type envelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newRouter(svc *Service) *gin.Engine {
	r := gin.New()
	noop := func(c *gin.Context) { c.Next() }
	NewHandler(svc).RegisterRoutes(r.Group("/api/auth"), middleware.Auth("secret"), noop)
	return r
}

func call(t *testing.T, r http.Handler, method, path string, body any, token string) (int, envelope) {
	t.Helper()
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), w.Body.String())
	return w.Code, env
}

func TestHandlerFlow(t *testing.T) {
	svc, _, db := newService(t)
	r := newRouter(svc)

	code, env := call(t, r, "POST", "/api/auth/register", map[string]string{"username": "b", "email": "x", "password": "1"}, "")
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "fail", env.Status)

	body := map[string]string{"username": "budi", "email": "budi@mail.id", "password": "rahasia1"}
	code, env = call(t, r, "POST", "/api/auth/register", body, "")
	assert.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "success", env.Status)
	assert.NotContains(t, string(env.Data), "password")

	code, _ = call(t, r, "POST", "/api/auth/register", body, "")
	assert.Equal(t, http.StatusConflict, code)

	code, _ = call(t, r, "POST", "/api/auth/login", "{bad", "")
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = call(t, r, "POST", "/api/auth/login", map[string]string{"username": "budi", "password": "salah"}, "")
	assert.Equal(t, http.StatusUnauthorized, code)

	code, env = call(t, r, "POST", "/api/auth/login", map[string]string{"username": "budi", "password": "rahasia1"}, "")
	require.Equal(t, http.StatusOK, code)
	var auth AuthResponse
	require.NoError(t, json.Unmarshal(env.Data, &auth))
	require.NotEmpty(t, auth.Token)
	assert.Equal(t, "budi", auth.User.Username)

	code, _ = call(t, r, "GET", "/api/auth/profile", nil, "")
	assert.Equal(t, http.StatusUnauthorized, code)
	code, env = call(t, r, "GET", "/api/auth/profile", nil, auth.Token)
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), `"username":"budi"`)

	code, _ = call(t, r, "PUT", "/api/auth/profile", map[string]string{"email": "bukan-email"}, auth.Token)
	assert.Equal(t, http.StatusBadRequest, code)
	code, env = call(t, r, "PUT", "/api/auth/profile", map[string]string{"username": "budi2"}, auth.Token)
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), `"username":"budi2"`)

	code, _ = call(t, r, "POST", "/api/auth/register",
		map[string]string{"username": "ani", "email": "ani@mail.id", "password": "rahasia1"}, "")
	require.Equal(t, http.StatusCreated, code)
	code, _ = call(t, r, "PUT", "/api/auth/profile", map[string]string{"username": "ani"}, auth.Token)
	assert.Equal(t, http.StatusConflict, code)

	code, env = call(t, r, "POST", "/api/auth/logout", nil, auth.Token)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "Logout berhasil", env.Message)

	// pengguna dihapus tetapi token masih berlaku -> 404
	require.NoError(t, db.Delete(&User{}, auth.User.ID).Error)
	code, _ = call(t, r, "GET", "/api/auth/profile", nil, auth.Token)
	assert.Equal(t, http.StatusNotFound, code)

	// database mati -> 500 pada seluruh endpoint yang menyentuh DB
	closeDB(t, db)
	code, env = call(t, r, "POST", "/api/auth/register",
		map[string]string{"username": "cici", "email": "cici@mail.id", "password": "rahasia1"}, "")
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "error", env.Status)
	code, _ = call(t, r, "POST", "/api/auth/login", map[string]string{"username": "ani", "password": "rahasia1"}, "")
	assert.Equal(t, http.StatusInternalServerError, code)
	code, _ = call(t, r, "GET", "/api/auth/profile", nil, auth.Token)
	assert.Equal(t, http.StatusInternalServerError, code)
	code, _ = call(t, r, "PUT", "/api/auth/profile", map[string]string{"username": "zzz"}, auth.Token)
	assert.Equal(t, http.StatusInternalServerError, code)
}
