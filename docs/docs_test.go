package docs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/swaggo/swag"
)

func TestEmbeddedSpecIsValidAndComplete(t *testing.T) {
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(swaggerJSON), &doc))
	assert.Equal(t, "2.0", doc["swagger"])

	paths := doc["paths"].(map[string]any)
	for _, p := range []string{
		"/api/public/categories", "/api/public/posts", "/api/public/posts/{slug}",
		"/api/auth/register", "/api/auth/login", "/api/auth/logout", "/api/auth/profile",
		"/api/posts", "/api/posts/stats", "/api/posts/{id}", "/api/posts/{id}/toggle-publish",
		"/api/posts/ai-generate", "/api/admin/users", "/api/admin/posts", "/api/admin/users/{id}/posts",
	} {
		assert.Contains(t, paths, p)
	}
}

func TestGetSwaggerJSONInjectsHostAndScheme(t *testing.T) {
	var doc struct {
		Host    string   `json:"host"`
		Schemes []string `json:"schemes"`
	}
	require.NoError(t, json.Unmarshal([]byte(GetSwaggerJSON("localhost:8081", "http")), &doc))
	assert.Equal(t, "localhost:8081", doc.Host)
	assert.Equal(t, []string{"http"}, doc.Schemes)
}

func TestSpecRegisteredForSwaggerUI(t *testing.T) {
	doc, err := swag.ReadDoc()
	require.NoError(t, err)
	assert.Equal(t, swaggerJSON, doc)
}
