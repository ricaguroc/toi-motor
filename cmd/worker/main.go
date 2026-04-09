package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/ricaguroc/toi-motor/internal/indexing"
	natspkg "github.com/ricaguroc/toi-motor/internal/platform/nats"
	"github.com/ricaguroc/toi-motor/internal/platform/ollama"
	"github.com/ricaguroc/toi-motor/internal/platform/postgres"
)

const (
	streamName   = "RECORDS"
	consumerName = "indexing-worker"
	subject      = "record.ingested"
)

func main() {
	// Structured logging — all subsequent log output uses JSON.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// 1. Load .env if present; ignore when missing.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		slog.Warn("worker: .env not loaded", "error", err)
	}

	databaseURL := mustEnv("DATABASE_URL")
	natsURL := mustEnv("NATS_URL")
	ollamaURL := mustEnv("OLLAMA_HOST")
	ollamaModel := envOr("EMBEDDING_MODEL", "nomic-embed-text")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()

	// 2. Connect PostgreSQL.
	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		log.Fatalf("worker: connect postgres: %v", err)
	}
	defer pool.Close()
	slog.Info("worker: postgres connected")

	// 3. Connect NATS + ensure stream exists.
	nc, err := natspkg.Connect(natsURL)
	if err != nil {
		log.Fatalf("worker: connect nats: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("worker: nats jetstream init: %v", err)
	}

	if err := natspkg.EnsureStream(js); err != nil {
		log.Fatalf("worker: nats ensure stream: %v", err)
	}
	slog.Info("worker: nats connected and stream ready")

	// 4. Create OllamaEmbedder.
	embedder := ollama.NewEmbedder(ollamaURL, ollamaModel)

	// 5. Create PostgresRecordStore.
	recordStore := postgres.NewPostgresRecordStore(pool)

	// 6. Create PostgresEmbeddingRepo.
	embeddingRepo := postgres.NewPostgresEmbeddingRepo(pool)

	// 7. Create DefaultChunker.
	chunker := indexing.DefaultChunker{}

	// 8. Create Pipeline.
	pipeline := indexing.NewPipeline(recordStore, chunker, embedder, embeddingRepo)

	// 9. Create IndexingWorker.
	worker := indexing.NewIndexingWorker(pipeline)

	// 10. Start consumer loop (blocks until ctx is cancelled).
	slog.Info("worker: starting consumer", "stream", streamName, "consumer", consumerName, "subject", subject)
	if err := natspkg.StartConsumer(ctx, js, streamName, consumerName, subject, worker.Handle); err != nil {
		log.Fatalf("worker: consumer error: %v", err)
	}

	slog.Info("worker: shutdown complete")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("worker: required env var %q is not set", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
