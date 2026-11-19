// Package docs menyediakan spesifikasi OpenAPI/Swagger untuk Swagger UI.
//
// Sumber kebenaran adalah swagger.json (di-embed ke biner sehingga tidak perlu
// berkas tambahan saat deploy). Spesifikasi ditulis manual, bukan hasil `swag init`.
package docs

import (
	_ "embed"
	"encoding/json"

	"github.com/swaggo/swag"
)

//go:embed swagger.json
var swaggerJSON string

type spec struct{}

// ReadDoc dipanggil oleh gin-swagger untuk menyajikan /swagger/doc.json.
func (spec) ReadDoc() string { return swaggerJSON }

func init() {
	swag.Register(swag.Name, spec{})
}

// GetSwaggerJSON mengembalikan spesifikasi dengan host dan scheme sesuai pemanggil,
// sehingga tombol "Try it out" di Swagger UI selalu mengarah ke server yang benar.
func GetSwaggerJSON(host, scheme string) string {
	var doc map[string]any
	_ = json.Unmarshal([]byte(swaggerJSON), &doc) // swaggerJSON selalu valid (diuji)
	doc["host"] = host
	doc["schemes"] = []string{scheme}
	out, _ := json.MarshalIndent(doc, "", "  ")
	return string(out)
}
