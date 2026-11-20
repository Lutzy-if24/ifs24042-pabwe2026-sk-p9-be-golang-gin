package database

import (
	"errors"
	"strings"
	"testing"

	"app/internal/config"
	"app/internal/features/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func memDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := OpenSQLite(":memory:")
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

func closeDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

func testConfig() *config.Config {
	return &config.Config{AdminUsername: "admin", AdminPassword: "admin"}
}

func TestConnectFailsWhenPostgresUnreachable(t *testing.T) {
	cfg := &config.Config{
		DBHost: "127.0.0.1", DBPort: "1", DBUsername: "u", DBPassword: "p",
		DBDatabase: "d", DBSSLMode: "disable",
	}
	_, err := Connect(cfg)
	assert.Error(t, err)
}

func TestMigrate(t *testing.T) {
	db := memDB(t)
	require.NoError(t, Migrate(db))
	assert.True(t, db.Migrator().HasTable("users"))
	assert.True(t, db.Migrator().HasTable("posts"))

	closeDB(t, db)
	assert.Error(t, Migrate(db))
}

func TestSeedCreatesAccountsIdempotently(t *testing.T) {
	db := memDB(t)
	require.NoError(t, Migrate(db))

	require.NoError(t, Seed(db, testConfig()))
	require.NoError(t, Seed(db, testConfig()))

	var users []auth.User
	require.NoError(t, db.Order("id").Find(&users).Error)
	require.Len(t, users, 2)
	assert.Equal(t, "admin", users[0].Username)
	assert.Equal(t, auth.RoleAdmin, users[0].Role)
	assert.Equal(t, "member", users[1].Username)
	assert.Equal(t, auth.RoleMember, users[1].Role)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(users[0].Password), []byte("admin")))
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(users[1].Password), []byte("member")))
}

func TestSeedErrors(t *testing.T) {
	t.Run("database tertutup", func(t *testing.T) {
		db := memDB(t)
		require.NoError(t, Migrate(db))
		closeDB(t, db)
		assert.Error(t, Seed(db, testConfig()))
	})

	t.Run("password terlalu panjang untuk bcrypt", func(t *testing.T) {
		db := memDB(t)
		require.NoError(t, Migrate(db))
		cfg := &config.Config{AdminUsername: "admin", AdminPassword: strings.Repeat("a", 80)}
		assert.Error(t, Seed(db, cfg))
	})

	t.Run("gagal menyimpan akun (email bentrok)", func(t *testing.T) {
		db := memDB(t)
		require.NoError(t, Migrate(db))
		require.NoError(t, db.Create(&auth.User{Username: "orang-lain", Email: "member@delcom.org", Password: "x"}).Error)
		assert.Error(t, Seed(db, testConfig()))
	})

	t.Run("gagal memperbarui akun yang sudah ada", func(t *testing.T) {
		db := memDB(t)
		require.NoError(t, Migrate(db))
		require.NoError(t, Seed(db, testConfig())) // akun sudah ada

		require.NoError(t, db.Callback().Update().Before("gorm:update").
			Register("test:force_update_error", func(tx *gorm.DB) {
				_ = tx.AddError(errors.New("update diblokir"))
			}))

		assert.Error(t, Seed(db, testConfig())) // jalur db.Save() gagal
	})
}