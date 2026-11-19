# Delcom Blog — Backend (Golang Gin)

REST API platform blog multi-tier dengan **AI Blog Generator** yang terintegrasi **Search Engine**.
Arsitektur Clean Architecture + Feature-First, GORM + PostgreSQL 17, JWT, Swagger UI, TDD (target 100% statement coverage).

## Struktur

```
cmd/server/main.go            entry point
docs/                         spesifikasi OpenAPI (swagger.json di-embed ke biner)
internal/config               loader .env bergaya Laravel
internal/database             koneksi PostgreSQL/SQLite, AutoMigrate, seeder
internal/middleware           CORS, Auth JWT, RequireRole, RateLimiter, ParseID
internal/response             envelope {status, message, data?}
internal/features/auth        registrasi, login (username/email), profil
internal/features/post        CRUD artikel, slug unik, reading time, stats, toggle publish, admin
internal/features/ai          klien Search Engine + OpenAI, generator artikel
internal/server               router, Swagger UI, /api.json, NoRoute 404
```

## Persiapan

```bash
# 1. Database
psql -U postgres -c "CREATE DATABASE pabwe_p9_blog;"

# 2. Dependensi (versi sesuai modul praktikum)
go get github.com/gin-gonic/gin@v1.12.0
go get gorm.io/gorm@v1.31.2
go get gorm.io/driver/postgres@v1.6.3
go get github.com/glebarez/sqlite@v1.11.0
go get github.com/golang-jwt/jwt/v5@v5.3.1
go get golang.org/x/crypto@v0.59.0
go get github.com/joho/godotenv@v1.5.1
go get github.com/swaggo/gin-swagger@v1.6.1
go get github.com/swaggo/files@v1.0.1
go get github.com/swaggo/swag@v1.16.6
go get github.com/stretchr/testify@v1.12.1
go mod tidy

# 3. Environment
cp .env.example .env      # sesuaikan DB_*, OPENAI_KEY, OPENAI_MODEL
```

> Bila salah satu versi di atas ditolak, jalankan `go get <modul>@latest` untuk modul itu lalu `go mod tidy`.

## Menjalankan

```bash
go run cmd/server/main.go
# [GIN-debug] Listening and serving HTTP on :8081
# Swagger UI: http://localhost:8081/swagger/index.html
```

Akun seeder: `admin / admin` (admin) dan `member / member` (member).

## Pengujian

```bash
go test -v -cover ./internal/...

# laporan per fungsi bila ada paket < 100%
go test -coverprofile=coverage.out ./internal/...
go tool cover -func=coverage.out | grep -v "100.0%"
go tool cover -html=coverage.out
```

Test tidak memerlukan PostgreSQL, Search Engine, maupun OpenAI: database memakai SQLite in-memory,
sedangkan layanan eksternal di-mock dengan `httptest.Server`.

## Build

```bash
go build -o server.exe cmd/server/main.go   # Windows
go build -o server cmd/server/main.go       # Linux/macOS
```
Berkas `.env` harus berada di direktori yang sama saat biner dijalankan.

## Endpoint

| Method | Endpoint | Akses |
|---|---|---|
| GET | `/swagger/*any`, `/api.json` | Publik |
| GET | `/api/public/categories` | Publik |
| GET | `/api/public/posts?q=&category=` | Publik |
| GET | `/api/public/posts/:slug` | Publik |
| POST | `/api/auth/register`, `/api/auth/login` | Publik (rate limit 30/menit) |
| POST | `/api/auth/logout` | Login |
| GET/PUT | `/api/auth/profile` | Login |
| GET/POST | `/api/posts` | Penulis |
| GET | `/api/posts/stats` | Penulis |
| GET/PUT/DELETE | `/api/posts/:id` | Penulis (milik sendiri) |
| PATCH | `/api/posts/:id/toggle-publish` | Penulis |
| POST | `/api/posts/ai-generate` | Penulis (rate limit 10/menit) |
| GET | `/api/admin/users`, `/api/admin/posts`, `/api/admin/users/:id/posts` | Admin |

Format respons: `{"status": "success|fail|error", "message": "...", "data": ...}`
(2xx / 4xx / 5xx). Rute tidak dikenal → 404 `Endpoint not found`.

## Keputusan desain

- **Slug unik**: `slugify(judul)`, bila bentrok menjadi `-2`, `-3`, dst. Slug **tidak berubah** saat judul diedit agar tautan publik stabil.
- **Reading time**: `ceil(jumlah kata / 200)`, minimal 1 menit.
- **Statistik**: `total`, `total_published`, `total_draft` (= kartu Total, Terbit, Draf).
- **Kepemilikan**: penulis hanya bisa melihat/mengubah/menghapus artikelnya; artikel orang lain dijawab 404.
- **Toggle publish**: mengisi `published_at` saat terbit, mengosongkannya saat kembali ke draf.
- **AI**: Search Engine yang gagal **tidak** menggagalkan generate; artikel tetap dibuat dan `search_warning` diisi. Kegagalan OpenAI → 502.
- **Login**: field `username` menerima username maupun email.
- **JWT**: HS256, masa berlaku 24 jam, algoritma lain ditolak.

## Hal yang perlu disesuaikan

- **Skema Search Engine Delcom** belum diketahui. `internal/features/ai/search.go` mengasumsikan `GET {endpoint}?q=...`
  dengan balasan `{"results":[{"title","url","snippet"}]}` (atau kunci `data`). Ubah `SearchClient.Search` bila berbeda.
- **Swagger** ditulis manual (`docs/swagger.json`), bukan dari `swag init`. Jangan menjalankan `swag init` kecuali
  Anda menambahkan anotasi di handler, karena akan menimpa `docs/docs.go`.
