package post

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/features/auth"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var errBoom = errors.New("boom")

func init() { gin.SetMode(gin.TestMode) }

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&auth.User{}, &Post{}))
	return db
}

func closeDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

func seedUser(t *testing.T, db *gorm.DB, name, role string) auth.User {
	t.Helper()
	u := auth.User{Username: name, Email: name + "@mail.id", Password: "x", Role: role}
	require.NoError(t, db.Create(&u).Error)
	return u
}

// failRepo membungkus Repository nyata dan gagal pada metode tertentu.
type failRepo struct {
	Repository
	failOn string
}

func (f failRepo) Create(p *Post) error {
	if f.failOn == "Create" {
		return errBoom
	}
	return f.Repository.Create(p)
}

func (f failRepo) Update(p *Post) error {
	if f.failOn == "Update" {
		return errBoom
	}
	return f.Repository.Update(p)
}

func (f failRepo) Delete(id uint) error {
	if f.failOn == "Delete" {
		return errBoom
	}
	return f.Repository.Delete(id)
}

func (f failRepo) FindOwned(id, userID uint) (*Post, error) {
	if f.failOn == "FindOwned" {
		return nil, errBoom
	}
	return f.Repository.FindOwned(id, userID)
}

func (f failRepo) FindByID(id uint) (*Post, error) {
	if f.failOn == "FindByID" {
		return nil, errBoom
	}
	return f.Repository.FindByID(id)
}

func (f failRepo) FindPublishedBySlug(slug string) (*Post, error) {
	if f.failOn == "FindPublishedBySlug" {
		return nil, errBoom
	}
	return f.Repository.FindPublishedBySlug(slug)
}

func (f failRepo) SlugExists(slug string) (bool, error) {
	if f.failOn == "SlugExists" {
		return false, errBoom
	}
	return f.Repository.SlugExists(slug)
}

func (f failRepo) Stats(userID uint) (int64, int64, error) {
	if f.failOn == "Stats" {
		return 0, 0, errBoom
	}
	return f.Repository.Stats(userID)
}

func failing(repo Repository, on string) *Service {
	return NewService(failRepo{Repository: repo, failOn: on})
}

// ---------- Fungsi murni ----------

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Belajar Golang & Gin!":         "belajar-golang-gin",
		"  Halo   Dunia  ":              "halo-dunia",
		"2026: Tahun AI":                "2026-tahun-ai",
		"!!!":                           "artikel",
		"":                              "artikel",
		strings.Repeat("a", 300):        strings.Repeat("a", 200),
		strings.Repeat("a", 199) + "-b": strings.Repeat("a", 199),
	}
	for in, want := range cases {
		assert.Equal(t, want, Slugify(in))
	}
}

func TestReadingTime(t *testing.T) {
	assert.Equal(t, 1, ReadingTime(""))
	assert.Equal(t, 1, ReadingTime("satu dua tiga"))
	assert.Equal(t, 1, ReadingTime(strings.Repeat("kata ", 200)))
	assert.Equal(t, 2, ReadingTime(strings.Repeat("kata ", 201)))
	assert.Equal(t, 5, ReadingTime(strings.Repeat("kata ", 1000)))
}

func TestCategoryHelpers(t *testing.T) {
	assert.True(t, IsValidCategory("Teknologi"))
	assert.False(t, IsValidCategory("Olahraga"))

	c, err := normalizeCategory("  ")
	require.NoError(t, err)
	assert.Equal(t, DefaultCategory, c)
	c, err = normalizeCategory(" Bisnis ")
	require.NoError(t, err)
	assert.Equal(t, "Bisnis", c)
	_, err = normalizeCategory("Olahraga")
	assert.ErrorIs(t, err, ErrInvalidCategory)
}

func TestToResponse(t *testing.T) {
	now := time.Now()
	p := &Post{ID: 1, Title: "T", Slug: "t", Content: "isi artikel", PublishedAt: &now}

	r := ToResponse(p, false)
	assert.Empty(t, r.Content)
	assert.Nil(t, r.Author)
	assert.Equal(t, 1, r.ReadingTime)

	p.User = auth.User{ID: 9, Username: "budi"}
	r = ToResponse(p, true)
	assert.Equal(t, "isi artikel", r.Content)
	require.NotNil(t, r.Author)
	assert.Equal(t, "budi", r.Author.Username)

	assert.NotNil(t, ToResponses(nil, false))
	assert.Len(t, ToResponses([]Post{*p, *p}, false), 2)

	// Uji fungsi publik
	pub := ToPublicResponse(p, true)
	assert.Equal(t, uint(0), pub.UserID) // UserID harus 0
	assert.Len(t, ToPublicResponses([]Post{*p, *p}, false), 2)
}

// ---------- Repository ----------

func TestRepository(t *testing.T) {
	db := newDB(t)
	repo := NewRepository(db)
	budi := seedUser(t, db, "budi", auth.RoleMember)
	ani := seedUser(t, db, "ani", auth.RoleMember)
	now := time.Now()

	p1 := &Post{UserID: budi.ID, Title: "Golang Gin", Slug: "golang-gin", Category: "Teknologi", Excerpt: "api cepat", Content: "belajar gin", IsPublished: true, PublishedAt: &now}
	p2 := &Post{UserID: budi.ID, Title: "Draf Rahasia", Slug: "draf-rahasia", Category: "Umum", Content: "belum terbit"}
	p3 := &Post{UserID: ani.ID, Title: "Kuliah Daring", Slug: "kuliah-daring", Category: "Pendidikan", Content: "tips KULIAH", IsPublished: true, PublishedAt: &now}
	for _, p := range []*Post{p1, p2, p3} {
		require.NoError(t, repo.Create(p))
	}

	got, err := repo.FindByID(p1.ID)
	require.NoError(t, err)
	assert.Equal(t, "budi", got.User.Username)

	_, err = repo.FindOwned(p1.ID, ani.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	owned, err := repo.FindOwned(p1.ID, budi.ID)
	require.NoError(t, err)
	assert.Equal(t, "Golang Gin", owned.Title)

	_, err = repo.FindPublishedBySlug("draf-rahasia")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	pub, err := repo.FindPublishedBySlug("golang-gin")
	require.NoError(t, err)
	assert.Equal(t, "budi", pub.User.Username)

	all, err := repo.ListPublished("", "")
	require.NoError(t, err)
	assert.Len(t, all, 2)
	byCat, err := repo.ListPublished("", "Pendidikan")
	require.NoError(t, err)
	require.Len(t, byCat, 1)
	assert.Equal(t, "kuliah-daring", byCat[0].Slug)
	search, err := repo.ListPublished("  KULIAH ", "")
	require.NoError(t, err)
	require.Len(t, search, 1)
	none, err := repo.ListPublished("tidak-ada", "Teknologi")
	require.NoError(t, err)
	assert.Empty(t, none)

	mine, err := repo.ListByUser(budi.ID)
	require.NoError(t, err)
	assert.Len(t, mine, 2)
	everything, err := repo.ListAll()
	require.NoError(t, err)
	assert.Len(t, everything, 3)

	exists, err := repo.SlugExists("golang-gin")
	require.NoError(t, err)
	assert.True(t, exists)
	exists, err = repo.SlugExists("baru")
	require.NoError(t, err)
	assert.False(t, exists)

	total, published, err := repo.Stats(budi.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, int64(1), published)

	p2.Title = "Draf Diubah"
	require.NoError(t, repo.Update(p2))
	again, err := repo.FindOwned(p2.ID, budi.ID)
	require.NoError(t, err)
	assert.Equal(t, "Draf Diubah", again.Title)

	users, err := repo.AdminUsers()
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, "budi", users[0].Username)
	assert.Equal(t, int64(2), users[0].TotalPosts)
	assert.Equal(t, int64(1), users[0].TotalPublished)
	assert.Equal(t, int64(1), users[1].TotalPosts)

	require.NoError(t, repo.Delete(p2.ID))
	_, err = repo.FindOwned(p2.ID, budi.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestRepositoryDatabaseErrors(t *testing.T) {
	db := newDB(t)
	repo := NewRepository(db)
	closeDB(t, db)

	assert.Error(t, repo.Create(&Post{Title: "x", Slug: "x"}))
	assert.Error(t, repo.Update(&Post{ID: 1}))
	assert.Error(t, repo.Delete(1))
	_, err := repo.FindByID(1)
	assert.Error(t, err)
	_, err = repo.FindOwned(1, 1)
	assert.Error(t, err)
	_, err = repo.FindPublishedBySlug("x")
	assert.Error(t, err)
	_, err = repo.ListPublished("", "")
	assert.Error(t, err)
	_, err = repo.ListByUser(1)
	assert.Error(t, err)
	_, err = repo.ListAll()
	assert.Error(t, err)
	_, err = repo.SlugExists("x")
	assert.Error(t, err)
	_, _, err = repo.Stats(1)
	assert.Error(t, err)
	_, err = repo.AdminUsers()
	assert.Error(t, err)
}

// ---------- Service ----------

func newService(t *testing.T) (*Service, Repository, *gorm.DB, auth.User) {
	t.Helper()
	db := newDB(t)
	repo := NewRepository(db)
	u := seedUser(t, db, "budi", auth.RoleMember)
	svc := NewService(repo)
	svc.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	return svc, repo, db, u
}

func newPost(title string) CreatePostRequest {
	return CreatePostRequest{Title: title, Content: "isi artikel"}
}

func TestCreatePost(t *testing.T) {
	svc, repo, _, u := newService(t)

	draft, err := svc.CreatePost(u.ID, newPost("Halo Dunia"))
	require.NoError(t, err)
	assert.Equal(t, "halo-dunia", draft.Slug)
	assert.Equal(t, DefaultCategory, draft.Category)
	assert.False(t, draft.IsPublished)
	assert.Nil(t, draft.PublishedAt)

	dup, err := svc.CreatePost(u.ID, newPost("Halo Dunia"))
	require.NoError(t, err)
	assert.Equal(t, "halo-dunia-2", dup.Slug)
	dup3, err := svc.CreatePost(u.ID, newPost("Halo Dunia"))
	require.NoError(t, err)
	assert.Equal(t, "halo-dunia-3", dup3.Slug)

	req := newPost("  Terbit  ")
	req.IsPublished = true
	req.Category = "Bisnis"
	published, err := svc.CreatePost(u.ID, req)
	require.NoError(t, err)
	assert.True(t, published.IsPublished)
	require.NotNil(t, published.PublishedAt)
	assert.Equal(t, 2026, published.PublishedAt.Year())
	assert.Equal(t, "Terbit", published.Title)

	bad := newPost("X")
	bad.Category = "Olahraga"
	_, err = svc.CreatePost(u.ID, bad)
	assert.ErrorIs(t, err, ErrInvalidCategory)

	_, err = failing(repo, "SlugExists").CreatePost(u.ID, newPost("Y"))
	assert.ErrorIs(t, err, errBoom)
	_, err = failing(repo, "Create").CreatePost(u.ID, newPost("Z"))
	assert.ErrorIs(t, err, errBoom)
}

func TestUpdatePost(t *testing.T) {
	svc, repo, db, u := newService(t)
	other := seedUser(t, db, "ani", auth.RoleMember)
	created, err := svc.CreatePost(u.ID, newPost("Judul Lama"))
	require.NoError(t, err)

	updated, err := svc.UpdatePost(u.ID, created.ID, UpdatePostRequest{Title: " Judul Baru ", Category: "Opini", Excerpt: "e", Content: "c", CoverImagePath: "/img.png"})
	require.NoError(t, err)
	assert.Equal(t, "Judul Baru", updated.Title)
	assert.Equal(t, "judul-lama", updated.Slug, "slug tetap stabil")
	assert.Equal(t, "Opini", updated.Category)
	assert.Equal(t, "/img.png", updated.CoverImagePath)

	// Error Forbidden (403) jika milik orang lain
	_, err = svc.UpdatePost(other.ID, created.ID, UpdatePostRequest{Title: "x", Content: "y"})
	assert.ErrorIs(t, err, ErrPostForbidden)

	_, err = svc.UpdatePost(u.ID, created.ID, UpdatePostRequest{Title: "x", Content: "y", Category: "Olahraga"})
	assert.ErrorIs(t, err, ErrInvalidCategory)
	_, err = failing(repo, "FindOwned").UpdatePost(u.ID, created.ID, UpdatePostRequest{Title: "x", Content: "y"})
	assert.ErrorIs(t, err, errBoom)
	_, err = failing(repo, "Update").UpdatePost(u.ID, created.ID, UpdatePostRequest{Title: "x", Content: "y"})
	assert.ErrorIs(t, err, errBoom)
}

func TestTogglePublish(t *testing.T) {
	svc, repo, db, u := newService(t)
	other := seedUser(t, db, "ani", auth.RoleMember)
	created, err := svc.CreatePost(u.ID, newPost("Toggle"))
	require.NoError(t, err)

	on, err := svc.TogglePublish(u.ID, created.ID)
	require.NoError(t, err)
	assert.True(t, on.IsPublished)
	require.NotNil(t, on.PublishedAt)

	off, err := svc.TogglePublish(u.ID, created.ID)
	require.NoError(t, err)
	assert.False(t, off.IsPublished)
	assert.Nil(t, off.PublishedAt)

	_, err = svc.TogglePublish(other.ID, created.ID)
	assert.ErrorIs(t, err, ErrPostForbidden)
	_, err = failing(repo, "Update").TogglePublish(u.ID, created.ID)
	assert.ErrorIs(t, err, errBoom)
}

func TestDeleteAndGetOwned(t *testing.T) {
	svc, repo, db, u := newService(t)
	other := seedUser(t, db, "ani", auth.RoleMember)
	created, err := svc.CreatePost(u.ID, newPost("Hapus"))
	require.NoError(t, err)

	got, err := svc.GetOwned(u.ID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Hapus", got.Title)
	_, err = svc.GetOwned(other.ID, created.ID)
	assert.ErrorIs(t, err, ErrPostForbidden)

	assert.ErrorIs(t, svc.DeletePost(other.ID, created.ID), ErrPostForbidden)
	assert.ErrorIs(t, failing(repo, "Delete").DeletePost(u.ID, created.ID), errBoom)
	require.NoError(t, svc.DeletePost(u.ID, created.ID))
	_, err = svc.GetOwned(u.ID, created.ID)
	assert.ErrorIs(t, err, ErrPostNotFound)
}

func TestStatsAndLists(t *testing.T) {
	svc, repo, _, u := newService(t)
	for i, title := range []string{"A", "B", "C"} {
		req := newPost(title)
		req.IsPublished = i == 0
		_, err := svc.CreatePost(u.ID, req)
		require.NoError(t, err)
	}

	stats, err := svc.Stats(u.ID)
	require.NoError(t, err)
	// PERBAIKAN: Nama field disesuaikan dengan dto.go baru
	assert.Equal(t, StatsResponse{TotalArticles: 3, TotalSelesai: 1, BelumSelesai: 2}, *stats)
	
	_, err = failing(repo, "Stats").Stats(u.ID)
	assert.ErrorIs(t, err, errBoom)

	mine, err := svc.ListMine(u.ID)
	require.NoError(t, err)
	assert.Len(t, mine, 3)
	pub, err := svc.PublicList("", " ")
	require.NoError(t, err)
	assert.Len(t, pub, 1)
	detail, err := svc.PublicDetail("a")
	require.NoError(t, err)
	assert.Equal(t, "A", detail.Title)
	_, err = svc.PublicDetail("b")
	assert.ErrorIs(t, err, ErrPostNotFound)
	_, err = failing(repo, "FindPublishedBySlug").PublicDetail("a")
	assert.ErrorIs(t, err, errBoom)

	users, err := svc.AdminUsers()
	require.NoError(t, err)
	assert.Len(t, users, 1)
	posts, err := svc.AdminPosts()
	require.NoError(t, err)
	assert.Len(t, posts, 3)
	userPosts, err := svc.AdminUserPosts(u.ID)
	require.NoError(t, err)
	assert.Len(t, userPosts, 3)
}

// ---------- Handler ----------

type envelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newRouter(svc *Service) *gin.Engine {
	r := gin.New()
	inject := func(c *gin.Context) {
		id := uint(0)
		if h := c.GetHeader("X-Test-User"); h != "" {
			for _, ch := range h {
				id = id*10 + uint(ch-'0')
			}
		}
		middleware.SetUser(c, middleware.AuthUser{ID: id, Username: "u", Role: c.GetHeader("X-Test-Role")})
		c.Next()
	}
	NewHandler(svc).RegisterRoutes(r.Group("/api"), inject, middleware.RequireRole(auth.RoleAdmin))
	return r
}

func call(t *testing.T, r http.Handler, method, path string, body any, userID, role string) (int, envelope) {
	t.Helper()
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", userID)
	req.Header.Set("X-Test-Role", role)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), w.Body.String())
	return w.Code, env
}

func TestHandlerAuthorFlow(t *testing.T) {
	svc, _, db, budi := newService(t)
	ani := seedUser(t, db, "ani", auth.RoleMember)
	r := newRouter(svc)
	bid := itoa(budi.ID)
	aid := itoa(ani.ID)

	code, env := call(t, r, "GET", "/api/public/categories", nil, "", "")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), "Teknologi")

	code, _ = call(t, r, "POST", "/api/posts", "{bad", bid, "member")
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = call(t, r, "POST", "/api/posts", map[string]string{"title": "X", "content": "y", "category": "Olahraga"}, bid, "member")
	assert.Equal(t, http.StatusBadRequest, code)

	code, env = call(t, r, "POST", "/api/posts", map[string]any{"title": "Artikel Satu", "content": "isi", "category": "Teknologi"}, bid, "member")
	require.Equal(t, http.StatusCreated, code)
	var created PostResponse
	require.NoError(t, json.Unmarshal(env.Data, &created))
	assert.Equal(t, "artikel-satu", created.Slug)
	id := itoa(created.ID)

	code, env = call(t, r, "GET", "/api/posts", nil, bid, "member")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), "artikel-satu")
	code, env = call(t, r, "GET", "/api/posts/stats", nil, bid, "member")
	assert.Equal(t, http.StatusOK, code)
	// PERBAIKAN: Cari string "belum_selesai"
	assert.Contains(t, string(env.Data), `"belum_selesai":1`)

	code, env = call(t, r, "GET", "/api/posts/"+id, nil, bid, "member")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), `"content":"isi"`)
	code, _ = call(t, r, "GET", "/api/posts/"+id, nil, aid, "member")
	assert.Equal(t, http.StatusForbidden, code) // PERBAIKAN: 403 Forbidden
	code, _ = call(t, r, "GET", "/api/posts/abc", nil, bid, "member")
	assert.Equal(t, http.StatusBadRequest, code)

	code, _ = call(t, r, "PUT", "/api/posts/"+id, "{bad", bid, "member")
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = call(t, r, "PUT", "/api/posts/"+id, map[string]string{"title": "Baru", "content": "isi baru"}, aid, "member")
	assert.Equal(t, http.StatusForbidden, code) // PERBAIKAN: 403 Forbidden
	code, env = call(t, r, "PUT", "/api/posts/"+id, map[string]string{"title": "Judul Baru", "content": "isi baru", "category": "Opini"}, bid, "member")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), "Judul Baru")

	code, env = call(t, r, "GET", "/api/public/posts", nil, "", "")
	assert.Equal(t, http.StatusOK, code)
	assert.JSONEq(t, "[]", string(env.Data))
	code, _ = call(t, r, "GET", "/api/public/posts/artikel-satu", nil, "", "")
	assert.Equal(t, http.StatusNotFound, code)

	code, env = call(t, r, "PATCH", "/api/posts/"+id+"/toggle-publish", nil, bid, "member")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), `"is_published":true`)
	code, _ = call(t, r, "PATCH", "/api/posts/"+id+"/toggle-publish", nil, aid, "member")
	assert.Equal(t, http.StatusForbidden, code) // PERBAIKAN: 403 Forbidden

	code, env = call(t, r, "GET", "/api/public/posts?q=baru&category=Opini", nil, "", "")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), "artikel-satu")
	assert.NotContains(t, string(env.Data), `"content"`)
	code, env = call(t, r, "GET", "/api/public/posts/artikel-satu", nil, "", "")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), `"author":{"id":`+bid)
	assert.Contains(t, string(env.Data), `"content":"isi baru"`)

	code, _ = call(t, r, "DELETE", "/api/posts/"+id, nil, aid, "member")
	assert.Equal(t, http.StatusForbidden, code) // PERBAIKAN: 403 Forbidden
	code, _ = call(t, r, "DELETE", "/api/posts/"+id, nil, bid, "member")
	assert.Equal(t, http.StatusOK, code)
}

func TestHandlerAdmin(t *testing.T) {
	svc, _, _, budi := newService(t)
	r := newRouter(svc)
	bid := itoa(budi.ID)

	_, err := svc.CreatePost(budi.ID, newPost("Milik Budi"))
	require.NoError(t, err)

	for _, path := range []string{"/api/admin/users", "/api/admin/posts", "/api/admin/users/" + bid + "/posts"} {
		code, _ := call(t, r, "GET", path, nil, bid, "member")
		assert.Equal(t, http.StatusForbidden, code, path)
	}

	code, env := call(t, r, "GET", "/api/admin/users", nil, "1", "admin")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), `"total_articles":1`)
	code, env = call(t, r, "GET", "/api/admin/posts", nil, "1", "admin")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), "milik-budi")
	code, env = call(t, r, "GET", "/api/admin/users/"+bid+"/posts", nil, "1", "admin")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), "milik-budi")
	
	// PERBAIKAN: Handler sekarang mengembalikan 200 OK (list kosong) karena ParseID menerima angka "0"
	code, _ = call(t, r, "GET", "/api/admin/users/0/posts", nil, "1", "admin")
	assert.Equal(t, http.StatusOK, code) 
}

func TestHandlerDatabaseDown(t *testing.T) {
	svc, _, db, budi := newService(t)
	r := newRouter(svc)
	bid := itoa(budi.ID)
	closeDB(t, db)

	valid := map[string]string{"title": "T", "content": "C"}
	cases := []struct {
		method, path string
		body         any
		role         string
	}{
		{"GET", "/api/public/posts", nil, ""},
		{"GET", "/api/public/posts/x", nil, ""},
		{"GET", "/api/posts", nil, "member"},
		{"GET", "/api/posts/stats", nil, "member"},
		{"GET", "/api/posts/1", nil, "member"},
		{"POST", "/api/posts", valid, "member"},
		{"PUT", "/api/posts/1", valid, "member"},
		{"PATCH", "/api/posts/1/toggle-publish", nil, "member"},
		{"DELETE", "/api/posts/1", nil, "member"},
		{"GET", "/api/admin/users", nil, "admin"},
		{"GET", "/api/admin/posts", nil, "admin"},
		{"GET", "/api/admin/users/1/posts", nil, "admin"},
	}
	for _, tc := range cases {
		code, env := call(t, r, tc.method, tc.path, tc.body, bid, tc.role)
		assert.Equal(t, http.StatusInternalServerError, code, tc.method+" "+tc.path)
		assert.Equal(t, "error", env.Status)
	}
}

func itoa(n uint) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
