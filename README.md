# TOI Motor — Universal Record Engine

Ingest any data. Query in natural language.

TOI is an engine that transforms heterogeneous data into searchable knowledge using embeddings, RAG, and LLMs. Open source, API-first, LLM-agnostic.

```bash
$ toi ingest file invoices.csv --source erp --type invoice
✓ 1,847 records ingested in 2.3s

$ toi query "what shipments were delayed last week?"
Found 12 records across 3 sources...

1. Shipment #LP-4821 — delayed 2 days
   Source: ERP · Recorded: 2026-03-12
```

---

## How it works

```
┌──────────────────────────────────────────────────────────────┐
│                         SOURCES                              │
│  CSV · ERP · Scanners · Forms · Email · APIs · Sensors       │
└────────────────────────────┬─────────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │    CLI / API     │  toi ingest file, POST /records
                    └────────┬────────┘
                             │
              ┌──────────────▼──────────────┐
              │    Universal Engine (TOI)    │
              │                             │
              │  01 INGESTION               │
              │  Flexible universal record   │
              │  Append-only, immutable      │
              │  SHA-256 checksum            │
              │                             │
              │  02 INDEXING                 │
              │  Chunking + Embeddings       │
              │  Async pipeline via NATS     │
              │  pgvector with HNSW          │
              │                             │
              │  03 QUERY                    │
              │  LLM-agnostic RAG            │
              │  Natural language            │
              │  Answers with evidence       │
              └─────────────────────────────┘
```

---

## Quick start

### Prerequisites

- Docker and Docker Compose

That's it. Everything runs containerized.

### 1. Clone and deploy

```bash
git clone https://github.com/ricaguroc/toi-motor.git
cd toi-motor
make deploy
```

`make deploy` builds the Go binaries, pulls the Ollama embedding model, runs database migrations, and starts the full stack. One command.

### 2. Create an API key

```bash
# Enter the running API container
docker compose exec api /bin/sh

# Inside the container, run the key creation script
# (or connect to postgres directly)
```

Or if you have Go installed locally:

```bash
make dev-infra   # start only infrastructure
go run scripts/create-apikey.go -name "default"
```

### 3. Try it out

Replace `YOUR_API_KEY` with the key from step 2.

#### Ingest a record

```bash
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "warehouse",
    "record_type": "scan",
    "entity_ref": "lot:LP-4821",
    "actor_ref": "operator:jperez",
    "title": "Incoming scan — lot LP-4821",
    "payload": {
      "temperature_c": 4.2,
      "weight_kg": 1250.5,
      "seal_intact": true,
      "dock": "D3"
    },
    "tags": ["cold-chain", "incoming"]
  }'
```

Response:

```json
{
  "record_id": "a1b2c3d4-...",
  "ingested_at": "2026-04-09T10:30:00Z",
  "checksum": "sha256:ab3f..."
}
```

#### Ingest more records (build some context)

```bash
# Temperature alert on same lot
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "sensor_dock_3",
    "record_type": "alert",
    "entity_ref": "lot:LP-4821",
    "title": "Temperature excursion detected",
    "payload": {
      "temperature_c": 9.8,
      "threshold_c": 8.0,
      "duration_min": 14,
      "zone": "cold-storage-A"
    },
    "tags": ["cold-chain", "alert", "temperature"]
  }'

# Movement to quality hold
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "wms",
    "record_type": "movement",
    "entity_ref": "lot:LP-4821",
    "actor_ref": "operator:mgarcia",
    "title": "Lot moved to quality hold",
    "payload": {
      "from_location": "cold-storage-A",
      "to_location": "quality-hold-1",
      "reason": "temperature excursion pending review"
    },
    "tags": ["quality", "hold"]
  }'

# A different lot — normal operation
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "warehouse",
    "record_type": "scan",
    "entity_ref": "lot:LP-5010",
    "actor_ref": "operator:jperez",
    "title": "Incoming scan — lot LP-5010",
    "payload": {
      "temperature_c": 3.1,
      "weight_kg": 980.0,
      "seal_intact": true,
      "dock": "D1"
    },
    "tags": ["cold-chain", "incoming"]
  }'

# Equipment maintenance record
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "maintenance",
    "record_type": "work_order",
    "entity_ref": "equipment:COMP-A01",
    "actor_ref": "tech:rlopez",
    "title": "Compressor A01 — scheduled maintenance",
    "payload": {
      "work_type": "preventive",
      "findings": "Refrigerant low, recharged to spec. Filter replaced.",
      "downtime_min": 45,
      "parts_used": ["filter-FK200", "refrigerant-R404A"]
    },
    "tags": ["maintenance", "cold-chain", "compressor"]
  }'
```

#### List records

```bash
# All records
curl -s http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" | jq

# Filter by entity
curl -s "http://localhost:8080/api/v1/records?entity_ref=lot:LP-4821" \
  -H "X-API-Key: YOUR_API_KEY" | jq

# Filter by type
curl -s "http://localhost:8080/api/v1/records?record_type=alert" \
  -H "X-API-Key: YOUR_API_KEY" | jq

# Filter by tag
curl -s "http://localhost:8080/api/v1/records?tag=cold-chain" \
  -H "X-API-Key: YOUR_API_KEY" | jq

# Filter by source
curl -s "http://localhost:8080/api/v1/records?source=maintenance" \
  -H "X-API-Key: YOUR_API_KEY" | jq
```

#### Semantic search (RAG, no LLM)

```bash
curl -s -X POST http://localhost:8080/api/v1/search \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"q": "temperature problems cold storage"}' | jq
```

#### Natural language query (RAG + LLM)

Requires `LLM_API_KEY` configured in `docker-compose.yml`.

```bash
curl -s -X POST http://localhost:8080/api/v1/query \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "q": "What happened with lot LP-4821?",
    "format": "conversational"
  }' | jq
```

Response:

```json
{
  "answer": "Lot LP-4821 arrived at dock D3 with a temperature of 4.2°C. A sensor later detected a temperature excursion to 9.8°C (threshold: 8.0°C) lasting 14 minutes in cold-storage-A. The lot was moved to quality-hold-1 pending review.",
  "confidence": "high",
  "records_cited": ["a1b2c3d4-...", "e5f6g7h8-...", "i9j0k1l2-..."],
  "gaps": null,
  "suggested_followup": [
    "Was the temperature excursion resolved?",
    "What other lots were in cold-storage-A during that time?"
  ],
  "retrieved_count": 3,
  "query_ms": 2340
}
```

#### Check system health

```bash
curl -s http://localhost:8080/health | jq
```

---

## Makefile commands

Everything you need runs through `make`:

| Command | What it does |
|---------|-------------|
| `make deploy` | Build images + start all services (full stack) |
| `make up` | Start services (pre-built, no rebuild) |
| `make down` | Stop all services |
| `make logs` | Tail logs from all services |
| `make logs-api` | Tail API logs only |
| `make logs-worker` | Tail worker logs only |
| `make test` | Run unit tests |
| `make bench` | Run all benchmarks (22 benchmarks across 3 packages) |
| `make test-integration` | Run integration tests |
| `make lint` | Run `go vet` |
| `make build` | Build Go binaries to `bin/` |
| `make clean` | Remove build artifacts |
| `make clean-all` | Remove everything including Docker volumes |

### Local development

If you want to iterate on Go code without rebuilding Docker images:

```bash
make dev-infra    # start only postgres, nats, ollama
make dev-api      # run API locally (in one terminal)
make dev-worker   # run worker locally (in another terminal)
```

---

## CLI

The `toi` CLI is a convenience layer over the API. It compiles to a single Go binary.

```bash
go build -o toi ./cmd/toi/
# or
make build   # builds all 3 binaries to bin/
```

| Command | Description |
|---------|-------------|
| `toi ingest file <path>` | Import CSV or JSON file as records |
| `toi ingest bulk <path>` | Import JSONL (one record per line) |
| `toi query <question>` | Natural language query (RAG + LLM) |
| `toi search <query>` | Semantic search without LLM |
| `toi watch <dir>` | Watch directory and auto-ingest new files |

**Configuration:**

| Flag | Environment variable | Default |
|------|---------------------|---------|
| `--api-url` | `TOI_API_URL` | `http://localhost:8080` |
| `--api-key` | `TOI_API_KEY` | (required) |
| `--source` | — | `cli` |
| `--type` | — | `document` |

**CSV mapping:** columns named `source`, `record_type`, `occurred_at`, `entity_ref`, `actor_ref`, `title`, and `tags` map to record fields. Everything else goes into `payload`.

---

## API reference

All endpoints under `/api/v1/*` require the `X-API-Key` header.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/records` | Ingest a record |
| `GET` | `/api/v1/records/:id` | Get a record by ID |
| `GET` | `/api/v1/records` | List records with filters |
| `POST` | `/api/v1/search` | Semantic search (RAG without LLM) |
| `POST` | `/api/v1/query` | Natural language query (RAG + LLM) |
| `GET` | `/health` | Infrastructure health (no auth required) |
| `GET` | `/health/live` | Liveness probe |
| `GET` | `/health/ready` | Readiness probe |

**List filters:** `entity_ref`, `actor_ref`, `record_type`, `source`, `tag`, `from`, `to`, `limit` (1-200), `cursor_time` + `cursor_id`.

---

## Data model

Everything that enters the engine is a **Record** — a universal, immutable, open-schema event:

```go
type Record struct {
    RecordID   uuid.UUID      // stable public identifier
    OccurredAt time.Time      // when it happened
    IngestedAt time.Time      // when it was recorded
    Source     string         // origin: "erp", "scanner", "manual"
    RecordType string         // type: "scan", "movement", "note"
    EntityRef  *string        // entity: "lot:L-2024-001", "equipment:PUMP-01"
    ActorRef   *string        // actor: "user:maria@company.com"
    Title      *string        // descriptive title
    Payload    map[string]any // free-form data (JSONB)
    ObjectRefs []string       // attached files
    Tags       []string       // labels
    Metadata   map[string]any // system metadata
    Checksum   string         // SHA-256 for tamper detection
}
```

**Immutability:** there are no Update or Delete operations. The store is append-only by design — enforced at the Go interface level, not by convention.

---

## Architecture

```
internal/
  record/       ← Domain: universal model, validation, checksum
  indexing/     ← Domain: indexing pipeline (text → chunk → embed → store)
  query/        ← Domain: query engine (RAG + LLM)
  auth/         ← Domain: API key authentication
  platform/     ← Infrastructure adapters
    postgres/   ← RecordStore, IndexStore, APIKeyStore
    nats/       ← EventPublisher, Consumer
    ollama/     ← Embedder (local)
    anthropic/  ← LanguageModel (Claude)
    http/       ← Handlers, Router, Middleware
cmd/
  api/          ← HTTP server
  worker/       ← NATS consumer for async indexing
  toi/          ← CLI for ingestion and queries
```

**Hexagonal architecture.** Domain packages define ports (interfaces). Adapters implement them. Any infrastructure component can be replaced without touching the engine.

**Infrastructure:**

| Service | Purpose |
|---------|---------|
| PostgreSQL 16 + pgvector | Relational + vector + FTS storage |
| NATS JetStream | Async indexing pipeline |
| Ollama | Local embeddings (nomic-embed-text, 768 dim) |

---

## Benchmarks

Run with `make bench`. Results on an i7-10700F:

| Benchmark | ops/sec | ns/op | allocs/op |
|-----------|---------|-------|-----------|
| Validate (minimal) | 186M | 6 ns | 0 |
| Checksum (50 keys) | 45K | 25 us | 169 |
| GenerateText (50 keys) | 62K | 19 us | 230 |
| Chunk (single record) | 14.9M | 87 ns | 1 |
| Full pipeline (100 keys) | 29K | 40 us | 437 |
| AssembleContext (50 chunks) | 13K | 90 us | 419 |
| BuildPrompt (32KB) | 167K | 7.6 us | 1 |

The Go domain layer processes ~25,000 records/second. The real bottleneck in production is Ollama (embedding generation), not the engine itself.

---

## Privacy

- **Embeddings:** local. Your data never leaves your infrastructure for vector generation.
- **LLM:** external and configurable. You choose the provider (Claude, GPT, Llama, local). Only queries travel to the external LLM, not full records.

For total privacy, implement the `LanguageModel` interface against a local model.

---

## Use cases

The engine is generic. The implementation is vertical.

| Vertical | What it records | Who uses it |
|----------|----------------|-------------|
| **Operational traceability** | Scans, movements, plant notes | Manufacturing, logistics |
| **Document compliance** | Regulatory evidence, certifications | Pharma, food |
| **Field auditing** | Inspections, photos, findings | Construction, energy |
| **Evidence management** | Documents, chain of custody | Legal, government |
| **Operational knowledge base** | Procedures, lessons learned | Any industry |

---

## Stack

| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.25 | Backend, CLI |
| PostgreSQL | 16 | 3-in-1 store (relational + pgvector + FTS) |
| pgvector | 0.7+ | Vector search with HNSW |
| NATS | 2.10 | Async messaging (JetStream) |
| Ollama | latest | Local embeddings |
| chi | v5 | HTTP router |
| Docker Compose | — | Full-stack orchestration |

---

## Contributing

TOI Motor is open source under the Apache 2.0 license. Contributions are welcome.

### How to contribute

1. **Fork** the repository
2. **Create a branch** for your feature or fix (`git checkout -b feat/my-feature`)
3. **Write tests** for new functionality
4. **Run the test suite** before submitting (`make test && make lint`)
5. **Open a pull request** with a clear description of what and why

### Areas where help is needed

- **Adapters for new LLMs** — implement the `LanguageModel` interface for providers beyond Claude
- **File format parsers** — extend the CLI to support more formats (Excel, Parquet, XML)
- **Embedding models** — add support for alternative embedding providers
- **Language support** — improve entity extraction for non-English text
- **Documentation** — tutorials, examples for specific verticals, deployment guides
- **Integration tests** — expand E2E coverage with realistic scenarios
- **Performance** — benchmark and optimize for high-throughput ingestion

### Development setup

```bash
git clone https://github.com/ricaguroc/toi-motor.git
cd toi-motor
make dev-infra       # start postgres, nats, ollama
make test            # run tests
make bench           # run benchmarks
make dev-api         # run API locally
make dev-worker      # run worker locally (separate terminal)
```

### Code conventions

- Hexagonal architecture: domain packages (`internal/record`, `internal/query`, `internal/indexing`) define interfaces; adapters (`internal/platform/*`) implement them
- Tests live next to the code they test (`*_test.go` in the same package)
- No `Update` or `Delete` on records — append-only by design
- All API endpoints under `/api/v1/*` require `X-API-Key`

---

## Documentation

- [Technical whitepaper](docs/whitepaper.md) — architecture, data model, full pipeline
- [Motor vs Adapters](docs/paper-motor-vs-adapters.md) — architectural separation and optimizations
- [Technical release v2](docs/release-v2.md) — current state, implemented API, E2E results

---

## License

Apache 2.0 — [see LICENSE](LICENSE)

No vendor lock-in. No black boxes. Auditable code.

---

> *Reality, when correctly recorded, can be queried as if it were a system.*
