package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeRoleAllBranches(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"admin huruf kecil", "admin", RoleAdmin},
		{"admin huruf besar", "ADMIN", RoleAdmin},
		{"admin campur huruf", "AdMiN", RoleAdmin},
		{"admin dengan spasi", "  admin  ", RoleAdmin},
		{"member", "member", RoleMember},
		{"kosong", "", RoleMember},
		{"hanya spasi", "   ", RoleMember},
		{"role tidak dikenal", "superuser", RoleMember},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeRole(tt.in))
		})
	}
}