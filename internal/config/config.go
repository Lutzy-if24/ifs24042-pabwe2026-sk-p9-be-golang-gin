// Package config memuat konfigurasi aplikasi dari environment / berkas .env
// dengan format penamaan mirip Laravel.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config menampung seluruh konfigurasi runtime aplikasi.
type Config struct {
	AppEnv  string
	AppPort string
	AppKey  string

	AdminUsername string
	AdminPassword string

	DBConnection string
	DBHost       string
	DBPort       string
	DBDatabase   string
	DBUsername   string
	DBPassword   string
	DBSSLMode    string

	SearchEngineEndpoint string
	OpenAIEndpoint       string
	OpenAIKey            string
	OpenAIModel          string
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// Load membaca berkas .env (jika ada) lalu variabel lingkungan dengan nilai default.
// Variabel lingkungan yang sudah ada tidak ditimpa oleh isi berkas .env.
func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		AppEnv:  env("APP_ENV", "development"),
		AppPort: env("APP_PORT", "8081"),
		AppKey:  env("APP_KEY", "delcom_blog_secret_key_super_secure_2026_random_string_32chars"),

		AdminUsername: env("ADMIN_USERNAME", "admin"),
		AdminPassword: env("ADMIN_PASSWORD", "admin"),

		DBConnection: env("DB_CONNECTION", "pgsql"),
		DBHost:       env("DB_HOST", "127.0.0.1"),
		DBPort:       env("DB_PORT", "5432"),
		DBDatabase:   env("DB_DATABASE", "pabwe_p9_blog"),
		DBUsername:   env("DB_USERNAME", "postgres"),
		DBPassword:   env("DB_PASSWORD", "postgres"),
		DBSSLMode:    env("DB_SSLMODE", "disable"),

		SearchEngineEndpoint: env("SEARCH_ENGINE_ENDPOINT", "https://delcom.org/api/v1/search"),
		OpenAIEndpoint:       env("OPENAI_ENDPOINT", "https://delcom.org/api/v1"),
		OpenAIKey:            env("OPENAI_KEY", ""),
		OpenAIModel:          env("OPENAI_MODEL", ""),
	}
}

// DSN menghasilkan connection string PostgreSQL.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
		c.DBHost, c.DBPort, c.DBUsername, c.DBPassword, c.DBDatabase, c.DBSSLMode,
	)
}

// Addr menghasilkan alamat listen server HTTP, contoh ":8081".
func (c *Config) Addr() string {
	return ":" + c.AppPort
}
