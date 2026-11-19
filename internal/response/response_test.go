package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

func newCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestSuccessWithAndWithoutData(t *testing.T) {
	c, w := newCtx()
	Success(c, http.StatusCreated, "dibuat", gin.H{"id": 1})
	assert.Equal(t, http.StatusCreated, w.Code)
	m := decode(t, w)
	assert.Equal(t, "success", m["status"])
	assert.Equal(t, "dibuat", m["message"])
	assert.Contains(t, m, "data")

	c, w = newCtx()
	Success(c, http.StatusOK, "ok", nil)
	assert.NotContains(t, decode(t, w), "data")
}

func TestFailAndError(t *testing.T) {
	c, w := newCtx()
	Fail(c, http.StatusNotFound, "tidak ada")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "fail", decode(t, w)["status"])

	c, w = newCtx()
	Error(c, http.StatusInternalServerError, "rusak")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "error", decode(t, w)["status"])
}

func TestAbortVariants(t *testing.T) {
	c, w := newCtx()
	AbortFail(c, http.StatusUnauthorized, "no")
	assert.True(t, c.IsAborted())
	assert.Equal(t, "fail", decode(t, w)["status"])

	c, w = newCtx()
	AbortError(c, http.StatusInternalServerError, "boom")
	assert.True(t, c.IsAborted())
	assert.Equal(t, "error", decode(t, w)["status"])
}

type sample struct {
	Name  string `validate:"required"`
	Email string `validate:"email"`
	Pass  string `validate:"min=6"`
	Short string `validate:"max=2"`
	Code  string `validate:"alphanum"`
}

func TestBindErrorValidation(t *testing.T) {
	err := validator.New().Struct(sample{Email: "x", Pass: "abc", Short: "abcdef", Code: "!!"})
	require.Error(t, err)

	c, w := newCtx()
	BindError(c, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	msg := decode(t, w)["message"].(string)
	for _, want := range []string{
		"name wajib diisi",
		"email harus berupa alamat email yang valid",
		"pass minimal 6 karakter",
		"short maksimal 2 karakter",
		"code tidak valid",
	} {
		assert.Contains(t, msg, want)
	}
}

func TestBindErrorMalformedJSON(t *testing.T) {
	c, w := newCtx()
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
	var dst sample
	err := c.ShouldBindJSON(&dst)
	require.Error(t, err)

	BindError(c, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Format request tidak valid", decode(t, w)["message"])
}
