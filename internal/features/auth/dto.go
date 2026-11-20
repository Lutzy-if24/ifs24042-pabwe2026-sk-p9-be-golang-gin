package auth

import "time"

// RegisterRequest adalah payload registrasi penulis baru.
// Role bersifat opsional: hanya "admin" atau "member" yang diterima,
// nilai lain (atau kosong) dipaksa menjadi "member" di service.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=6,max=72"`
	Role     string `json:"role"`
}

// LoginRequest adalah payload login. Username boleh berisi username maupun email.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UpdateProfileRequest adalah payload pembaruan profil; seluruh field opsional.
type UpdateProfileRequest struct {
	Username string `json:"username" binding:"omitempty,min=3,max=50"`
	Email    string `json:"email" binding:"omitempty,email,max=255"`
	Password string `json:"password" binding:"omitempty,min=6,max=72"`
}

// UserResponse adalah representasi publik pengguna.
type UserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthResponse dikembalikan setelah login berhasil.
type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// ToResponse memetakan User ke UserResponse.
func ToResponse(u *User) UserResponse {
	return UserResponse{ID: u.ID, Username: u.Username, Email: u.Email, Role: u.Role, CreatedAt: u.CreatedAt}
}