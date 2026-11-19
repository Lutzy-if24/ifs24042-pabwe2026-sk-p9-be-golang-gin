// Package server merangkai seluruh handler ke dalam router Gin.
package server

import (
	"net/http"
	"time"

	"app/docs"
	"app/internal/config"
	"app/internal/features/ai"
	"app/internal/features/auth"
	"app/internal/features/post"
	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

const tokenTTL = 24 * time.Hour

// NewRouter membangun engine Gin lengkap dengan middleware, rute API, Swagger UI dan handler 404.
func NewRouter(cfg *config.Config, db *gorm.DB, httpClient *http.Client) *gin.Engine {
	r := gin.New()
	// Tanpa proxy tepercaya: ClientIP (dipakai rate limiter) tidak bisa dipalsukan lewat X-Forwarded-For.
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS())

	// Dokumentasi
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/swagger/index.html") })
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/api.json", func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.GetSwaggerJSON(c.Request.Host, scheme)))
	})

	// Layanan
	authSvc := auth.NewService(auth.NewRepository(db), cfg.AppKey, tokenTTL)
	postSvc := post.NewService(post.NewRepository(db))
	aiSvc := ai.NewService(
		ai.NewSearchClient(cfg.SearchEngineEndpoint, httpClient),
		ai.NewChatClient(cfg.OpenAIEndpoint, cfg.OpenAIKey, cfg.OpenAIModel, httpClient),
	)

	authMW := middleware.Auth(cfg.AppKey)
	// PERBAIKAN: Memastikan role admin terbaca dengan benar
	adminMW := middleware.RequireRole(auth.RoleAdmin) 

	api := r.Group("/api")
	
	// Rute Auth
	auth.NewHandler(authSvc).RegisterRoutes(api.Group("/auth"), authMW, middleware.RateLimiter(30, time.Minute))
	
	// Rute Post & Admin
	post.NewHandler(postSvc).RegisterRoutes(api, authMW, adminMW)
	
	// Rute AI (dengan limit lebih ketat)
	ai.NewHandler(aiSvc).RegisterRoutes(api, authMW, middleware.RateLimiter(10, time.Minute))

	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, http.StatusNotFound, "Endpoint not found")
	})
	return r
}