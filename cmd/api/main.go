package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go/jetstream"
	natsgo "github.com/nats-io/nats.go"

	platformhttp "github.com/ricaguroc/toi-motor/internal/platform/http"
	natspkg "github.com/ricaguroc/toi-motor/internal/platform/nats"
	"github.com/ricaguroc/toi-motor/internal/platform/anthropic"
	"github.com/ricaguroc/toi-motor/internal/platform/ollama"
	"github.com/ricaguroc/toi-motor/internal/platform/postgres"
	"github.com/ricaguroc/toi-motor/internal/query"
	"github.com/ricaguroc/toi-motor/internal/record"
)

func main() {
	// Structured logging — all subsequent log output uses JSON.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load .env if present; ignore the error when the file does not exist.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		slog.Warn("api: .env not loaded", "error", err)
	}

	databaseURL := mustEnv("DATABASE_URL")
	apiPort := envOr("API_PORT", "8080")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		log.Fatalf("api: connect to postgres: %v", err)
	}
	defer pool.Close()

	migrationsPath := envOr("MIGRATIONS_PATH", resolvedMigrationsPath())

	switch err := postgres.RunMigrations(databaseURL, migrationsPath); {
	case err == nil:
		slog.Info("api: migrations ok")
	case errors.Is(err, migrate.ErrNoChange):
		slog.Info("api: migrations: no change")
	default:
		log.Fatalf("api: migrations: %v", err)
	}

	recordStore := postgres.NewPostgresRecordStore(pool)
	apiKeyStore := postgres.NewPostgresAPIKeyStore(pool)

	// Wire up NATS publisher if NATS_URL is configured.
	// Graceful degradation: if NATS is unavailable, we log and continue with nil publisher.
	var publisher record.EventPublisher
	var nc *natsgo.Conn
	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		var natsErr error
		nc, natsErr = natspkg.Connect(natsURL)
		if natsErr != nil {
			slog.Warn("api: NATS connect failed, event publishing disabled", "error", natsErr)
		} else {
			defer nc.Close()
			js, jsErr := jetstream.New(nc)
			if jsErr != nil {
				slog.Warn("api: NATS JetStream init failed, event publishing disabled", "error", jsErr)
			} else {
				if streamErr := natspkg.EnsureStream(js); streamErr != nil {
					slog.Warn("api: NATS EnsureStream failed, event publishing disabled", "error", streamErr)
				} else {
					publisher = natspkg.NewPublisher(js)
					slog.Info("api: NATS event publishing enabled")
				}
			}
		}
	} else {
		slog.Info("api: NATS_URL not set, event publishing disabled")
	}

	recordService := record.NewRecordService(recordStore, publisher)

	// --- Embeddings (Ollama — local, lightweight) ---
	// The embedding model runs locally via Ollama. It's small (~270MB) and fast.
	// This powers both /search (RAG-only) and /query (RAG + LLM).
	var retriever query.Retriever
	ollamaURL := os.Getenv("OLLAMA_HOST")
	if ollamaURL != "" {
		embeddingModel := envOr("EMBEDDING_MODEL", "nomic-embed-text")
		embedder := ollama.NewEmbedder(ollamaURL, embeddingModel)
		embeddingRepo := postgres.NewPostgresEmbeddingRepo(pool)
		retriever = query.NewRetriever(embedder, embeddingRepo)
		slog.Info("api: embeddings enabled", "ollama", ollamaURL, "model", embeddingModel)
	} else {
		slog.Info("api: OLLAMA_HOST not set, search and query disabled")
	}

	// --- LLM (Anthropic Claude — external, configurable) ---
	// The LLM is external and agnostic. Today it uses Anthropic's API.
	// If someone wants a different provider, they implement query.LanguageModel.
	var queryService *query.QueryService
	llmAPIKey := os.Getenv("LLM_API_KEY")
	if llmAPIKey != "" && retriever != nil {
		llmBaseURL := envOr("LLM_BASE_URL", "https://api.anthropic.com")
		llmModel := envOr("LLM_MODEL", "claude-sonnet-4-20250514")
		llmClient := anthropic.NewClient(llmBaseURL, llmAPIKey, llmModel)
		queryService = query.NewQueryService(retriever, llmClient)
		slog.Info("api: query engine enabled", "llm_base_url", llmBaseURL, "model", llmModel)
	} else if llmAPIKey == "" {
		slog.Info("api: LLM_API_KEY not set, /query disabled (search still available)")
	} else {
		slog.Info("api: embeddings not configured, /query disabled")
	}

	health := platformhttp.NewHealthChecker(pool, nc, ollamaURL)

	router := platformhttp.NewRouter(recordService, queryService, retriever, apiKeyStore, health)

	srv := &http.Server{
		Addr:         ":" + apiPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("api: listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("api: server: %v", err)
		}
	}()

	<-ctx.Done()
	slog.Info("api: shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("api: graceful shutdown failed", "error", err)
	}

	slog.Info("api: shutdown complete")
}

// resolvedMigrationsPath returns the absolute path to the migrations directory
// relative to this source file, so the binary works regardless of working dir.
func resolvedMigrationsPath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "migrations"
	}
	// cmd/api/main.go → ../../migrations
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("api: required env var %q is not set", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
