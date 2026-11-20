package post

import (
	"errors"
	"net/http"

	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler adalah controller endpoint /api/public/*, /api/posts/* dan /api/admin/*.
type Handler struct{ svc *Service }

// NewHandler membuat Handler artikel.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterRoutes mendaftarkan rute publik, penulis dan admin pada grup /api.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMW, adminMW gin.HandlerFunc) {
	pub := api.Group("/public")
	pub.GET("/categories", h.Categories)
	pub.GET("/posts", h.PublicList)
	pub.GET("/posts/:slug", h.PublicDetail)

	posts := api.Group("/posts", authMW)
	posts.GET("", h.Mine)
	posts.POST("", h.Create)
	posts.GET("/stats", h.Stats)
	posts.GET("/:id", middleware.ParseID(), h.Get)
	posts.PUT("/:id", middleware.ParseID(), h.Update)
	posts.PATCH("/:id/toggle-publish", middleware.ParseID(), h.TogglePublish)
	posts.DELETE("/:id", middleware.ParseID(), h.Delete)

	admin := api.Group("/admin", authMW, adminMW)
	admin.GET("/users", h.AdminUsers)
	admin.GET("/posts", h.AdminPosts)
	admin.GET("/users/:id/posts", middleware.ParseID(), h.AdminUserPosts)
}

func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPostNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrPostForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrInvalidCategory):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
	}
}

func currentUserID(c *gin.Context) uint {
	u, _ := middleware.CurrentUser(c)
	return u.ID
}

// Categories godoc: GET /api/public/categories
func (h *Handler) Categories(c *gin.Context) {
	response.Success(c, http.StatusOK, "Daftar kategori berhasil diambil", Categories)
}

// PublicList godoc: GET /api/public/posts?q=&category=
func (h *Handler) PublicList(c *gin.Context) {
	posts, err := h.svc.PublicList(c.Query("q"), c.Query("category"))
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar artikel berhasil diambil", ToPublicResponses(posts, false))
}

// PublicDetail godoc: GET /api/public/posts/:slug
func (h *Handler) PublicDetail(c *gin.Context) {
	p, err := h.svc.PublicDetail(c.Param("slug"))
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diambil", ToPublicResponse(p, true))
}

// Mine godoc: GET /api/posts
func (h *Handler) Mine(c *gin.Context) {
	posts, err := h.svc.ListMine(currentUserID(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar artikel berhasil diambil", ToResponses(posts, false))
}

// Stats godoc: GET /api/posts/stats
func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.svc.Stats(currentUserID(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Statistik berhasil diambil", stats)
}

// Get godoc: GET /api/posts/:id
func (h *Handler) Get(c *gin.Context) {
	p, err := h.svc.GetOwned(currentUserID(c), middleware.ParamID(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diambil", ToResponse(p, true))
}

// Create godoc: POST /api/posts
func (h *Handler) Create(c *gin.Context) {
	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	p, err := h.svc.CreatePost(currentUserID(c), req)
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "Artikel berhasil dibuat", ToResponse(p, true))
}

// Update godoc: PUT /api/posts/:id
func (h *Handler) Update(c *gin.Context) {
	userID := currentUserID(c)
	id := middleware.ParamID(c)

	// Cek keberadaan & kepemilikan dulu: 404 / 403 harus menang atas validasi body (400).
	if _, err := h.svc.GetOwned(userID, id); err != nil {
		fail(c, err)
		return
	}

	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	p, err := h.svc.UpdatePost(userID, id, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diperbarui", ToResponse(p, true))
}

// TogglePublish godoc: PATCH /api/posts/:id/toggle-publish
func (h *Handler) TogglePublish(c *gin.Context) {
	p, err := h.svc.TogglePublish(currentUserID(c), middleware.ParamID(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Status publikasi berhasil diubah", ToResponse(p, false))
}

// Delete godoc: DELETE /api/posts/:id
func (h *Handler) Delete(c *gin.Context) {
	if err := h.svc.DeletePost(currentUserID(c), middleware.ParamID(c)); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil dihapus", nil)
}

// AdminUsers godoc: GET /api/admin/users
func (h *Handler) AdminUsers(c *gin.Context) {
	users, err := h.svc.AdminUsers()
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar pengguna berhasil diambil", users)
}

// AdminPosts godoc: GET /api/admin/posts
func (h *Handler) AdminPosts(c *gin.Context) {
	posts, err := h.svc.AdminPosts()
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Arsip artikel berhasil diambil", ToResponses(posts, false))
}

// AdminUserPosts godoc: GET /api/admin/users/:id/posts
func (h *Handler) AdminUserPosts(c *gin.Context) {
	posts, err := h.svc.AdminUserPosts(middleware.ParamID(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel pengguna berhasil diambil", ToResponses(posts, false))
}