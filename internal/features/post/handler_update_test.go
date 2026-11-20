package post

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerUpdateServiceError(t *testing.T) {
	svc, repo, _, budi := newService(t)
	created, err := svc.CreatePost(budi.ID, newPost("Judul Awal"))
	require.NoError(t, err)
	bid := itoa(budi.ID)
	id := itoa(created.ID)

	// Kategori tidak valid -> 400
	r := newRouter(svc)
	code, _ := call(t, r, "PUT", "/api/posts/"+id,
		map[string]string{"title": "x", "content": "y", "category": "Olahraga"}, bid, "member")
	assert.Equal(t, http.StatusBadRequest, code)

	// Repository.Update gagal setelah kepemilikan lolos -> 500
	rFail := newRouter(failing(repo, "Update"))
	code, env := call(t, rFail, "PUT", "/api/posts/"+id,
		map[string]string{"title": "Judul Baru", "content": "isi baru"}, bid, "member")
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "error", env.Status)
}