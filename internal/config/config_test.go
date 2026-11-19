package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var allKeys = []string{
	"APP_ENV", "APP_PORT", "APP_KEY", "ADMIN_USERNAME", "ADMIN_PASSWORD",
	"DB_CONNECTION", "DB_HOST", "DB_PORT", "DB_DATABASE", "DB_USERNAME", "DB_PASSWORD", "DB_SSLMODE",
	"SEARCH_ENGINE_ENDPOINT", "OPENAI_ENDPOINT", "OPENAI_KEY", "OPENAI_MODEL",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range allKeys {
		t.Setenv(k, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	cfg := Load()

	assert.Equal(t, "development", cfg.AppEnv)
	assert.Equal(t, "8081", cfg.AppPort)
	assert.NotEmpty(t, cfg.AppKey)
	assert.Equal(t, "admin", cfg.AdminUsername)
	assert.Equal(t, "admin", cfg.AdminPassword)
	assert.Equal(t, "pgsql", cfg.DBConnection)
	assert.Equal(t, "127.0.0.1", cfg.DBHost)
	assert.Equal(t, "5432", cfg.DBPort)
	assert.Equal(t, "pabwe_p9_blog", cfg.DBDatabase)
	assert.Equal(t, "postgres", cfg.DBUsername)
	assert.Equal(t, "postgres", cfg.DBPassword)
	assert.Equal(t, "disable", cfg.DBSSLMode)
	assert.Equal(t, "https://delcom.org/api/v1/search", cfg.SearchEngineEndpoint)
	assert.Equal(t, "https://delcom.org/api/v1", cfg.OpenAIEndpoint)
	assert.Empty(t, cfg.OpenAIKey)
	assert.Empty(t, cfg.OpenAIModel)
}

func TestLoadFromEnvironment(t *testing.T) {
	clearEnv(t)
	t.Setenv("APP_PORT", "9999")
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("OPENAI_KEY", "secret")

	cfg := Load()

	assert.Equal(t, "9999", cfg.AppPort)
	assert.Equal(t, "db.internal", cfg.DBHost)
	assert.Equal(t, "secret", cfg.OpenAIKey)
}

func TestDSNAndAddr(t *testing.T) {
	cfg := &Config{
		AppPort: "8081", DBHost: "h", DBPort: "1", DBUsername: "u",
		DBPassword: "p", DBDatabase: "d", DBSSLMode: "disable",
	}

	assert.Equal(t, "host=h port=1 user=u password=p dbname=d sslmode=disable TimeZone=Asia/Jakarta", cfg.DSN())
	assert.Equal(t, ":8081", cfg.Addr())
}
