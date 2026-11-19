// Package database mengelola koneksi PostgreSQL (runtime) dan SQLite in-memory (test),
// migrasi skema otomatis serta seeder akun bawaan.
package database

import (
	"log"
	"os"
	"time"

	"app/internal/config"
	"app/internal/features/auth"
	"app/internal/features/post"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open membuka koneksi GORM dengan dialector apa pun.
func Open(dialector gorm.Dialector) (*gorm.DB, error) {
	return gorm.Open(dialector, &gorm.Config{
		Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	})
}

// Connect membuka koneksi PostgreSQL berdasarkan konfigurasi.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	return Open(postgres.Open(cfg.DSN()))
}

// OpenSQLite membuka SQLite (":memory:" untuk pengujian mandiri tanpa PostgreSQL).
func OpenSQLite(dsn string) (*gorm.DB, error) {
	return Open(sqlite.Open(dsn))
}

// Migrate menjalankan AutoMigrate untuk seluruh model.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&auth.User{}, &post.Post{})
}

type account struct {
	username, email, password, role string
}

// Seed membuat akun bawaan (admin dari .env dan member/member).
// PERBAIKAN: Jika akun sudah ada, role dan password dipaksa untuk diperbarui (diperbaiki),
// agar jika ada kesalahan role di database Render sebelumnya, akan tertimpa dengan yang benar.
func Seed(db *gorm.DB, cfg *config.Config) error {
	accounts := []account{
		{cfg.AdminUsername, cfg.AdminUsername + "@delcom.org", cfg.AdminPassword, auth.RoleAdmin},
		{"member", "member@delcom.org", "member", auth.RoleMember},
	}
	for _, a := range accounts {
		var u auth.User
		// Cari user berdasarkan username
		err := db.Where("username = ?", a.username).First(&u).Error
		
		hash, hashErr := auth.HashPassword(a.password)
		if hashErr != nil {
			return hashErr
		}

		if err == nil {
			// JIKA USER SUDAH ADA: Paksa update role dan password agar benar
			u.Role = a.role
			u.Password = hash
			if err := db.Save(&u).Error; err != nil {
				return err
			}
			continue
		}

		// JIKA USER BELUM ADA: Buat baru
		u = auth.User{Username: a.username, Email: a.email, Password: hash, Role: a.role}
		if err := db.Create(&u).Error; err != nil {
			return err
		}
	}
	return nil
}