package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

const secret = "test-secret"

func init() { gin.SetMode(gin.TestMode) }

func serve(r *gin.Engine, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func token(t *testing.T, method jwt.SigningMethod, key any, exp time.Time) string {
	t.Helper()
	claims := &Claims{
		UserID: 7, Username: "budi", Role: "member",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(exp)},
	}
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	assert.NoError(t, err)
	return s
}

func TestCORS(t *testing.T) {
	r := gin.New()
	r.Use(CORS())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.OPTIONS("/x", func(c *gin.Context) { c.String(http.StatusOK, "never") })

	w := serve(r, http.MethodGet, "/x", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")

	w = serve(r, http.MethodOptions, "/x", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAuth(t *testing.T) {
	r := gin.New()
	r.Use(Auth(secret))
	r.GET("/me", func(c *gin.Context) {
		u, ok := CurrentUser(c)
		c.JSON(http.StatusOK, gin.H{"ok": ok, "id": u.ID, "username": u.Username, "role": u.Role})
	})

	valid := token(t, jwt.SigningMethodHS256, []byte(secret), time.Now().Add(time.Hour))
	expired := token(t, jwt.SigningMethodHS256, []byte(secret), time.Now().Add(-time.Hour))
	wrongKey := token(t, jwt.SigningMethodHS256, []byte("other"), time.Now().Add(time.Hour))
	wrongAlg := token(t, jwt.SigningMethodHS512, []byte(secret), time.Now().Add(time.Hour))

	cases := []struct {
		name   string
		header string
		code   int
	}{
		{"tanpa header", "", http.StatusUnauthorized},
		{"skema salah", "Basic abc", http.StatusUnauthorized},
		{"token kosong", "Bearer   ", http.StatusUnauthorized},
		{"token acak", "Bearer not-a-jwt", http.StatusUnauthorized},
		{"kedaluwarsa", "Bearer " + expired, http.StatusUnauthorized},
		{"kunci salah", "Bearer " + wrongKey, http.StatusUnauthorized},
		{"algoritma tidak diizinkan", "Bearer " + wrongAlg, http.StatusUnauthorized},
		{"valid", "Bearer " + valid, http.StatusOK},
		{"skema huruf kecil", "bearer " + valid, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := serve(r, http.MethodGet, "/me", map[string]string{"Authorization": tc.header})
			assert.Equal(t, tc.code, w.Code)
			if tc.code == http.StatusOK {
				assert.Contains(t, w.Body.String(), `"username":"budi"`)
				assert.Contains(t, w.Body.String(), `"id":7`)
			}
		})
	}
}

func TestCurrentUserMissing(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, ok := CurrentUser(c)
	assert.False(t, ok)
}

func TestRequireRole(t *testing.T) {
	withUser := func(u *AuthUser) gin.HandlerFunc {
		return func(c *gin.Context) {
			if u != nil {
				SetUser(c, *u)
			}
			c.Next()
		}
	}
	build := func(u *AuthUser) *gin.Engine {
		r := gin.New()
		r.Use(withUser(u), RequireRole("admin", "editor"))
		r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
		return r
	}

	assert.Equal(t, http.StatusUnauthorized, serve(build(nil), http.MethodGet, "/x", nil).Code)
	assert.Equal(t, http.StatusForbidden, serve(build(&AuthUser{Role: "member"}), http.MethodGet, "/x", nil).Code)
	assert.Equal(t, http.StatusOK, serve(build(&AuthUser{Role: "admin"}), http.MethodGet, "/x", nil).Code)
	assert.Equal(t, http.StatusOK, serve(build(&AuthUser{Role: "editor"}), http.MethodGet, "/x", nil).Code)
}

func TestRateLimiter(t *testing.T) {
	current := time.Unix(1_000, 0)
	r := gin.New()
	r.Use(newRateLimiter(2, time.Minute, func() time.Time { return current }))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/x", nil).Code)
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/x", nil).Code)

	w := serve(r, http.MethodGet, "/x", nil)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))

	current = current.Add(61 * time.Second)
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/x", nil).Code)
}

func TestRateLimiterPublicConstructor(t *testing.T) {
	r := gin.New()
	r.Use(RateLimiter(1, time.Hour))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/x", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/x", nil).Code)
}

func TestParseID(t *testing.T) {
	r := gin.New()
	r.GET("/items/:id", ParseID(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": ParamID(c)})
	})

	w := serve(r, http.MethodGet, "/items/42", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":42`)

	for _, bad := range []string{"abc", "0", "-1", "99999999999"} {
		assert.Equal(t, http.StatusBadRequest, serve(r, http.MethodGet, "/items/"+bad, nil).Code, bad)
	}
}
