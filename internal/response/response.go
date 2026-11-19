// Package response menstandarkan format JSON seluruh endpoint REST API:
// {"status": "success|fail|error", "message": "...", "data": ...}.
package response

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Body adalah envelope respons standar.
type Body struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Success mengirim respons 2xx.
func Success(c *gin.Context, code int, message string, data any) {
	c.JSON(code, Body{Status: "success", Message: message, Data: data})
}

// Fail mengirim respons 4xx (kesalahan klien).
func Fail(c *gin.Context, code int, message string) {
	c.JSON(code, Body{Status: "fail", Message: message})
}

// Error mengirim respons 5xx (kesalahan server).
func Error(c *gin.Context, code int, message string) {
	c.JSON(code, Body{Status: "error", Message: message})
}

// AbortFail seperti Fail tetapi menghentikan rantai middleware.
func AbortFail(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, Body{Status: "fail", Message: message})
}

// AbortError seperti Error tetapi menghentikan rantai middleware.
func AbortError(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, Body{Status: "error", Message: message})
}

// BindError mengubah galat binding/validasi menjadi respons 400 yang ramah.
func BindError(c *gin.Context, err error) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		msgs := make([]string, 0, len(ve))
		for _, fe := range ve {
			msgs = append(msgs, fmt.Sprintf("%s %s", strings.ToLower(fe.Field()), ruleMessage(fe)))
		}
		Fail(c, http.StatusBadRequest, "Validasi gagal: "+strings.Join(msgs, "; "))
		return
	}
	Fail(c, http.StatusBadRequest, "Format request tidak valid")
}

func ruleMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "wajib diisi"
	case "email":
		return "harus berupa alamat email yang valid"
	case "min":
		return "minimal " + fe.Param() + " karakter"
	case "max":
		return "maksimal " + fe.Param() + " karakter"
	default:
		return "tidak valid"
	}
}
