package ai

import (
	"net/http"

	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler adalah controller endpoint POST /api/posts/ai-generate.
type Handler struct{ svc *Service }

// NewHandler membuat Handler AI.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterRoutes mendaftarkan rute AI pada grup /api (butuh login + pembatasan laju).
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMW, limiter gin.HandlerFunc) {
	api.POST("/posts/ai-generate", authMW, limiter, h.Generate)
}

// Generate godoc: POST /api/posts/ai-generate
func (h *Handler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	res, err := h.svc.Generate(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Draf artikel berhasil dibuat", res)
}
