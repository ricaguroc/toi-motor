// scripts/create-apikey.go — standalone script to create an API key.
// Usage: go run scripts/create-apikey.go -name "my-key"
//
// Connects to DATABASE_URL from .env or environment, generates a random
// API key, bcrypt-hashes it, inserts into api_keys table, and prints the
// raw key to stdout. The raw key is shown ONCE — it cannot be recovered.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	name := flag.String("name", "default", "Name for the API key")
	flag.Parse()

	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// Generate random 32-byte key
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		log.Fatalf("generate key: %v", err)
	}
	rawKey := hex.EncodeToString(rawBytes)
	prefix := rawKey[:8]

	// Bcrypt hash
	hash, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt hash: %v", err)
	}

	// Insert into database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx,
		`INSERT INTO api_keys (key_hash, key_prefix, name) VALUES ($1, $2, $3)`,
		string(hash), prefix, *name,
	)
	if err != nil {
		log.Fatalf("insert api key: %v", err)
	}

	fmt.Println("=== API KEY CREATED ===")
	fmt.Printf("Name:   %s\n", *name)
	fmt.Printf("Prefix: %s\n", prefix)
	fmt.Printf("Key:    %s\n", rawKey)
	fmt.Println("")
	fmt.Println("Save this key now — it cannot be recovered.")
	fmt.Printf("Usage:  curl -H 'Authorization: Bearer %s' http://localhost:8080/api/v1/records\n", rawKey)
}
