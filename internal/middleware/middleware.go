// Package middleware berisi CORS, autentikasi JWT, role guard, rate limiter
// dan validasi parameter :id.
package middleware

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"app/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	userKey = "user"
	idKey   = "id"
)

// Claims adalah isi token JWT.
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// AuthUser adalah identitas pengguna yang disuntikkan ke Gin context.
type AuthUser struct {
	ID       uint
	Username string
	Role     string
}

// SetUser menyimpan identitas pengguna ke context.
func SetUser(c *gin.Context, u AuthUser) {
	c.Set(userKey, u)
}

// CurrentUser mengambil identitas pengguna dari context.
func CurrentUser(c *gin.Context) (AuthUser, bool) {
	v, _ := c.Get(userKey)
	u, ok := v.(AuthUser)
	return u, ok
}

// CORS mengizinkan akses lintas-domain bagi frontend React.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
		h.Set("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth memverifikasi Bearer Token JWT (HS256) dan menyuntikkan AuthUser.
func Auth(secretKey string) gin.HandlerFunc {
	key := []byte(secretKey)
	keyFunc := func(*jwt.Token) (any, error) { return key, nil }
	return func(c *gin.Context) {
		parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			response.AbortFail(c, http.StatusUnauthorized, "Token autentikasi tidak ditemukan")
			return
		}
		claims := &Claims{}
		_, err := jwt.ParseWithClaims(strings.TrimSpace(parts[1]), claims, keyFunc, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil {
			response.AbortFail(c, http.StatusUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
			return
		}
		SetUser(c, AuthUser{ID: claims.UserID, Username: claims.Username, Role: claims.Role})
		c.Next()
	}
}

// RequireRole hanya meneruskan request dari pengguna dengan salah satu role yang diizinkan.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := CurrentUser(c)
		if !ok {
			response.AbortFail(c, http.StatusUnauthorized, "Autentikasi diperlukan")
			return
		}
		for _, r := range roles {
			if strings.EqualFold(r, u.Role) {
				c.Next()
				return
			}
		}
		// DEBUG sementara: hapus setelah masalah admin selesai.
		log.Printf("[DEBUG] akses ditolak (403): path=%s user_id=%d username=%q role=%q", c.Request.URL.Path, u.ID, u.Username, u.Role)
		response.AbortFail(c, http.StatusForbidden, "Anda tidak memiliki akses ke sumber daya ini")
	}
}

type visitor struct {
	count   int
	resetAt time.Time
}

// RateLimiter membatasi jumlah request per IP dalam jendela waktu tetap.
func RateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	return newRateLimiter(limit, window, time.Now)
}

func newRateLimiter(limit int, window time.Duration, now func() time.Time) gin.HandlerFunc {
	var mu sync.Mutex
	visitors := map[string]*visitor{}
	retryAfter := strconv.Itoa(int(window.Seconds()))

	return func(c *gin.Context) {
		ip := c.ClientIP()
		t := now()

		mu.Lock()
		v, ok := visitors[ip]
		if !ok || !t.Before(v.resetAt) {
			v = &visitor{resetAt: t.Add(window)}
			visitors[ip] = v
		}
		v.count++
		allowed := v.count <= limit
		mu.Unlock()

		if !allowed {
			c.Header("Retry-After", retryAfter)
			response.AbortFail(c, http.StatusTooManyRequests, "Terlalu banyak request, coba lagi nanti")
			return
		}
		c.Next()
	}
}

// ParseID memvalidasi parameter :id (bilangan bulat positif) dan menyimpannya ke context.
func ParseID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			response.AbortFail(c, http.StatusBadRequest, "ID tidak valid")
			return
		}
		c.Set(idKey, uint(id))
		c.Next()
	}
}

// ParamID mengambil nilai :id yang sudah divalidasi oleh ParseID.
func ParamID(c *gin.Context) uint {
	return c.GetUint(idKey)
}