package auth

import "gorm.io/gorm"

// Repository mengabstraksi akses data pengguna.
type Repository interface {
	Create(u *User) error
	FindByID(id uint) (*User, error)
	FindByLogin(login string) (*User, error)
	Exists(username, email string, excludeID uint) (bool, error)
	Update(u *User) error
}

type gormRepository struct{ db *gorm.DB }

// NewRepository membuat Repository berbasis GORM.
func NewRepository(db *gorm.DB) Repository { return &gormRepository{db: db} }

func (r *gormRepository) Create(u *User) error { return r.db.Create(u).Error }

func (r *gormRepository) Update(u *User) error { return r.db.Save(u).Error }

func first(q *gorm.DB) (*User, error) {
	var u User
	if err := q.First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *gormRepository) FindByID(id uint) (*User, error) {
	return first(r.db.Where("id = ?", id))
}

func (r *gormRepository) FindByLogin(login string) (*User, error) {
	return first(r.db.Where("username = ? OR email = ?", login, login))
}

// Exists memeriksa apakah username atau email sudah dipakai pengguna lain (selain excludeID).
func (r *gormRepository) Exists(username, email string, excludeID uint) (bool, error) {
	var n int64
	err := r.db.Model(&User{}).
		Where("(username = ? OR email = ?) AND id <> ?", username, email, excludeID).
		Count(&n).Error
	return n > 0, err
}
