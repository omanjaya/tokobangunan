// Program seed: insert master data awal (satuan, gudang, owner user) secara idempotent.
// Dijalankan via `make seed`. Password owner di-print sekali ke stdout saat user dibuat.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/argon2"
)

const (
	argonMemory  uint32 = 64 * 1024 // 64 MiB
	argonTime    uint32 = 3
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

type satuanSeed struct {
	kode string
	nama string
}

type gudangSeed struct {
	kode    string
	nama    string
	alamat  string
	telepon string
}

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer conn.Close(ctx)

	if err := seedSatuan(ctx, conn); err != nil {
		log.Fatalf("seed satuan: %v", err)
	}
	if err := seedGudang(ctx, conn); err != nil {
		log.Fatalf("seed gudang: %v", err)
	}
	if err := seedOwner(ctx, conn); err != nil {
		log.Fatalf("seed owner: %v", err)
	}

	log.Println("seed: done")
}

func seedSatuan(ctx context.Context, conn *pgx.Conn) error {
	data := []satuanSeed{
		{"sak", "Sak"},
		{"kg", "Kilogram"},
		{"batang", "Batang"},
		{"m", "Meter"},
		{"m2", "Meter Persegi"},
		{"lusin", "Lusin"},
		{"biji", "Biji"},
		{"roll", "Roll"},
		{"lembar", "Lembar"},
	}
	for _, s := range data {
		var existing int64
		err := conn.QueryRow(ctx, `SELECT id FROM satuan WHERE kode = $1`, s.kode).Scan(&existing)
		if err == nil {
			continue
		}
		if err != pgx.ErrNoRows {
			return fmt.Errorf("check satuan %s: %w", s.kode, err)
		}
		if _, err := conn.Exec(ctx,
			`INSERT INTO satuan (kode, nama) VALUES ($1, $2)`, s.kode, s.nama); err != nil {
			return fmt.Errorf("insert satuan %s: %w", s.kode, err)
		}
		log.Printf("seed: satuan %s inserted", s.kode)
	}
	return nil
}

func seedGudang(ctx context.Context, conn *pgx.Conn) error {
	data := []gudangSeed{
		{"CANGGU", "Cabang Canggu", "", ""},
		{"SAYAN", "Cabang Sayan", "", ""},
		{"PEJENG", "Cabang Pejeng", "", ""},
		{"SAMPLANGAN", "Cabang Samplangan", "", ""},
		{"TEGES", "Cabang Teges", "", ""},
	}
	for _, g := range data {
		var existing int64
		err := conn.QueryRow(ctx, `SELECT id FROM gudang WHERE kode = $1`, g.kode).Scan(&existing)
		if err == nil {
			continue
		}
		if err != pgx.ErrNoRows {
			return fmt.Errorf("check gudang %s: %w", g.kode, err)
		}
		var alamat, telepon any
		if g.alamat != "" {
			alamat = g.alamat
		}
		if g.telepon != "" {
			telepon = g.telepon
		}
		if _, err := conn.Exec(ctx,
			`INSERT INTO gudang (kode, nama, alamat, telepon) VALUES ($1, $2, $3, $4)`,
			g.kode, g.nama, alamat, telepon); err != nil {
			return fmt.Errorf("insert gudang %s: %w", g.kode, err)
		}
		log.Printf("seed: gudang %s inserted", g.kode)
	}
	return nil
}

func seedOwner(ctx context.Context, conn *pgx.Conn) error {
	const username = "owner"
	var existing int64
	err := conn.QueryRow(ctx, `SELECT id FROM "user" WHERE username = $1`, username).Scan(&existing)
	if err == nil {
		log.Printf("seed: owner already exists (id=%d), skipping. Password NOT re-printed.", existing)
		return nil
	}
	if err != pgx.ErrNoRows {
		return fmt.Errorf("check owner: %w", err)
	}

	password, err := generatePassword(16)
	if err != nil {
		return fmt.Errorf("generate password: %w", err)
	}
	hash, err := hashArgon2id(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO "user" (username, password_hash, nama_lengkap, email, role, gudang_id)
		VALUES ($1, $2, $3, NULL, 'owner', NULL)`,
		username, hash, "Owner"); err != nil {
		return fmt.Errorf("insert owner: %w", err)
	}

	fmt.Println("===========================================")
	fmt.Println("OWNER ACCOUNT CREATED")
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("Password: %s\n", password)
	fmt.Println("===========================================")
	fmt.Println("Save this password securely. It will NOT be shown again.")
	return nil
}

// generatePassword menghasilkan password alfanumerik random sepanjang n karakter.
func generatePassword(n int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, n)
	rnd := make([]byte, n)
	if _, err := rand.Read(rnd); err != nil {
		return "", err
	}
	for i := 0; i < n; i++ {
		buf[i] = alphabet[int(rnd[i])%len(alphabet)]
	}
	return string(buf), nil
}

// hashArgon2id menghasilkan encoded hash format PHC: $argon2id$v=19$m=...,t=...,p=...$salt$hash
func hashArgon2id(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}
