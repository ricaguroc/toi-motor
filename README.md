<<<<<<< HEAD
# TOI Motor — Universal Record Engine

Ingest any data. Query in natural language.

TOI is an engine that transforms heterogeneous data into searchable knowledge using embeddings, RAG, and LLMs. Open source, API-first, LLM-agnostic.
=======
# TOI Motor — Motor Universal de Registros

Ingesta cualquier dato. Consulta en lenguaje natural.

TOI es un motor que transforma datos heterogeneos en conocimiento buscable usando embeddings, RAG y LLMs. Open source, API-first, agnostico al proveedor de LLM.
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

```bash
$ toi ingest file invoices.csv --source erp --type invoice
✓ 1,847 records ingested in 2.3s

<<<<<<< HEAD
$ toi query "what shipments were delayed last week?"
Found 12 records across 3 sources...

1. Shipment #LP-4821 — delayed 2 days
   Source: ERP · Recorded: 2026-03-12
=======
$ toi query "¿que envios se retrasaron la semana pasada?"
Se encontraron 12 registros en 3 fuentes...

1. Envio #LP-4821 — retrasado 2 dias
   Fuente: ERP · Registrado: 2026-03-12
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
```

---

<<<<<<< HEAD
## How it works

```
┌──────────────────────────────────────────────────────────────┐
│                         SOURCES                              │
│  CSV · ERP · Scanners · Forms · Email · APIs · Sensors       │
=======
## Como funciona

```
┌──────────────────────────────────────────────────────────────┐
│                         FUENTES                              │
│  CSV · ERP · Escaners · Formularios · Email · APIs · Sensores│
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
└────────────────────────────┬─────────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │    CLI / API     │  toi ingest file, POST /records
                    └────────┬────────┘
                             │
              ┌──────────────▼──────────────┐
<<<<<<< HEAD
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
=======
              │    Motor Universal (TOI)     │
              │                              │
              │  01 INGESTA                  │
              │  Registro universal flexible │
              │  Append-only, inmutable      │
              │  Checksum SHA-256            │
              │                              │
              │  02 INDEXACION               │
              │  Chunking + Embeddings       │
              │  Pipeline async via NATS     │
              │  pgvector con HNSW           │
              │                              │
              │  03 CONSULTA                 │
              │  RAG + LLM agnostico         │
              │  Lenguaje natural            │
              │  Respuestas con evidencia    │
              └──────────────────────────────┘
```

---

## Inicio rapido
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

## Quick start

<<<<<<< HEAD
### Prerequisites

- Docker and Docker Compose

That's it. Everything runs containerized.

### 1. Clone and deploy
=======
- Docker y Docker Compose

Nada mas. Todo corre containerizado.

### 1. Clonar y desplegar
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

```bash
git clone https://github.com/ricaguroc/toi-motor.git
cd toi-motor
make deploy
```

<<<<<<< HEAD
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
=======
`make deploy` compila los binarios Go, descarga el modelo de embeddings de Ollama, ejecuta las migraciones de base de datos y levanta el stack completo. Un solo comando.

### 2. Crear una API key

```bash
# Entrar al contenedor de la API
docker compose exec api /bin/sh

# Dentro del contenedor, ejecutar el script de creacion de key
# (o conectarse directamente a postgres)
```

O si tenes Go instalado localmente:

```bash
make dev-infra   # levantar solo la infraestructura
go run scripts/create-apikey.go -name "default"
```

### 3. Probarlo

Reemplaza `YOUR_API_KEY` con la key del paso 2.

#### Ingestar un registro
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

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

<<<<<<< HEAD
Response:
=======
Respuesta:
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

```json
{
  "record_id": "a1b2c3d4-...",
  "ingested_at": "2026-04-09T10:30:00Z",
  "checksum": "sha256:ab3f..."
}
```

<<<<<<< HEAD
#### Ingest more records (build some context)

```bash
# Temperature alert on same lot
=======
#### Ingestar mas registros (construir contexto)

```bash
# Alerta de temperatura en el mismo lote
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "sensor_dock_3",
    "record_type": "alert",
    "entity_ref": "lot:LP-4821",
<<<<<<< HEAD
    "title": "Temperature excursion detected",
=======
    "title": "Excursion de temperatura detectada",
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
    "payload": {
      "temperature_c": 9.8,
      "threshold_c": 8.0,
      "duration_min": 14,
      "zone": "cold-storage-A"
    },
    "tags": ["cold-chain", "alert", "temperature"]
  }'

<<<<<<< HEAD
# Movement to quality hold
=======
# Movimiento a retencion de calidad
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "wms",
    "record_type": "movement",
    "entity_ref": "lot:LP-4821",
    "actor_ref": "operator:mgarcia",
<<<<<<< HEAD
    "title": "Lot moved to quality hold",
    "payload": {
      "from_location": "cold-storage-A",
      "to_location": "quality-hold-1",
      "reason": "temperature excursion pending review"
=======
    "title": "Lote movido a retencion de calidad",
    "payload": {
      "from_location": "cold-storage-A",
      "to_location": "quality-hold-1",
      "reason": "excursion de temperatura pendiente de revision"
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
    },
    "tags": ["quality", "hold"]
  }'

<<<<<<< HEAD
# A different lot — normal operation
=======
# Otro lote — operacion normal
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "warehouse",
    "record_type": "scan",
    "entity_ref": "lot:LP-5010",
    "actor_ref": "operator:jperez",
<<<<<<< HEAD
    "title": "Incoming scan — lot LP-5010",
=======
    "title": "Escaneo de ingreso lote LP-5010",
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
    "payload": {
      "temperature_c": 3.1,
      "weight_kg": 980.0,
      "seal_intact": true,
      "dock": "D1"
    },
    "tags": ["cold-chain", "incoming"]
  }'

<<<<<<< HEAD
# Equipment maintenance record
=======
# Registro de mantenimiento de equipo
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "maintenance",
    "record_type": "work_order",
    "entity_ref": "equipment:COMP-A01",
    "actor_ref": "tech:rlopez",
<<<<<<< HEAD
    "title": "Compressor A01 — scheduled maintenance",
    "payload": {
      "work_type": "preventive",
      "findings": "Refrigerant low, recharged to spec. Filter replaced.",
=======
    "title": "Compresor A01 — mantenimiento programado",
    "payload": {
      "work_type": "preventive",
      "findings": "Refrigerante bajo, recargado a especificacion. Filtro reemplazado.",
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
      "downtime_min": 45,
      "parts_used": ["filter-FK200", "refrigerant-R404A"]
    },
    "tags": ["maintenance", "cold-chain", "compressor"]
  }'
```

<<<<<<< HEAD
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
=======
#### Listar registros

```bash
# Todos los registros
curl -s http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" | jq

# Filtrar por entidad
curl -s "http://localhost:8080/api/v1/records?entity_ref=lot:LP-4821" \
  -H "X-API-Key: YOUR_API_KEY" | jq

# Filtrar por tipo
curl -s "http://localhost:8080/api/v1/records?record_type=alert" \
  -H "X-API-Key: YOUR_API_KEY" | jq

# Filtrar por tag
curl -s "http://localhost:8080/api/v1/records?tag=cold-chain" \
  -H "X-API-Key: YOUR_API_KEY" | jq

# Filtrar por fuente
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
curl -s "http://localhost:8080/api/v1/records?source=maintenance" \
  -H "X-API-Key: YOUR_API_KEY" | jq
```

<<<<<<< HEAD
#### Semantic search (RAG, no LLM)
=======
#### Busqueda semantica (RAG, sin LLM)
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

```bash
curl -s -X POST http://localhost:8080/api/v1/search \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
<<<<<<< HEAD
  -d '{"q": "temperature problems cold storage"}' | jq
```

#### Natural language query (RAG + LLM)

Requires `LLM_API_KEY` configured in `docker-compose.yml`.
=======
  -d '{"q": "problemas de temperatura en cold storage"}' | jq
```

#### Consulta en lenguaje natural (RAG + LLM)

Requiere `LLM_API_KEY` configurada en `docker-compose.yml`.
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

```bash
curl -s -X POST http://localhost:8080/api/v1/query \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
<<<<<<< HEAD
    "q": "What happened with lot LP-4821?",
=======
    "q": "¿Que paso con el lote LP-4821?",
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
    "format": "conversational"
  }' | jq
```

<<<<<<< HEAD
Response:

```json
{
  "answer": "Lot LP-4821 arrived at dock D3 with a temperature of 4.2°C. A sensor later detected a temperature excursion to 9.8°C (threshold: 8.0°C) lasting 14 minutes in cold-storage-A. The lot was moved to quality-hold-1 pending review.",
=======
Respuesta:

```json
{
  "answer": "El lote LP-4821 llego al muelle D3 con una temperatura de 4.2°C. Un sensor detecto posteriormente una excursion de temperatura a 9.8°C (umbral: 8.0°C) durante 14 minutos en cold-storage-A. El lote fue movido a quality-hold-1 pendiente de revision.",
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
  "confidence": "high",
  "records_cited": ["a1b2c3d4-...", "e5f6g7h8-...", "i9j0k1l2-..."],
  "gaps": null,
  "suggested_followup": [
<<<<<<< HEAD
    "Was the temperature excursion resolved?",
    "What other lots were in cold-storage-A during that time?"
=======
    "¿Se resolvio la excursion de temperatura?",
    "¿Que otros lotes estaban en cold-storage-A durante ese periodo?"
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
  ],
  "retrieved_count": 3,
  "query_ms": 2340
}
```

<<<<<<< HEAD
#### Check system health
=======
#### Verificar salud del sistema
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

```bash
curl -s http://localhost:8080/health | jq
```

---

<<<<<<< HEAD
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
=======
## Comandos del Makefile

Todo lo que necesitas se ejecuta con `make`:

| Comando | Que hace |
|---------|----------|
| `make deploy` | Construye imagenes + levanta todos los servicios (stack completo) |
| `make up` | Levanta servicios (sin reconstruir) |
| `make down` | Detiene todos los servicios |
| `make logs` | Tail de logs de todos los servicios |
| `make logs-api` | Tail de logs solo de la API |
| `make logs-worker` | Tail de logs solo del worker |
| `make test` | Ejecuta tests unitarios |
| `make bench` | Ejecuta todos los benchmarks (22 benchmarks en 3 paquetes) |
| `make test-integration` | Ejecuta tests de integracion |
| `make lint` | Ejecuta `go vet` |
| `make build` | Compila binarios Go a `bin/` |
| `make clean` | Elimina artefactos de compilacion |
| `make clean-all` | Elimina todo incluyendo volumenes Docker |

### Desarrollo local

Si queres iterar sobre el codigo Go sin reconstruir imagenes Docker:

```bash
make dev-infra    # levantar solo postgres, nats, ollama
make dev-api      # correr API localmente (en una terminal)
make dev-worker   # correr worker localmente (en otra terminal)
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
```

---

## CLI

<<<<<<< HEAD
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
=======
El CLI `toi` es una capa de conveniencia sobre la API. Compila como un unico binario Go.

```bash
go build -o toi ./cmd/toi/
# o
make build   # compila los 3 binarios a bin/
```

| Comando | Descripcion |
|---------|-------------|
| `toi ingest file <path>` | Importa CSV o JSON como registros |
| `toi ingest bulk <path>` | Importa JSONL (un registro por linea) |
| `toi query <pregunta>` | Consulta en lenguaje natural (RAG + LLM) |
| `toi search <query>` | Busqueda semantica sin LLM |
| `toi watch <dir>` | Monitorea carpeta y auto-ingesta archivos nuevos |

**Configuracion:**

| Flag | Variable de entorno | Default |
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
|------|---------------------|---------|
| `--api-url` | `TOI_API_URL` | `http://localhost:8080` |
| `--api-key` | `TOI_API_KEY` | (required) |
| `--source` | — | `cli` |
| `--type` | — | `document` |

**CSV mapping:** columns named `source`, `record_type`, `occurred_at`, `entity_ref`, `actor_ref`, `title`, and `tags` map to record fields. Everything else goes into `payload`.

---

<<<<<<< HEAD
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
=======
## Referencia de API

Todos los endpoints bajo `/api/v1/*` requieren el header `X-API-Key`.

| Metodo | Endpoint | Descripcion |
|--------|----------|-------------|
| `POST` | `/api/v1/records` | Ingestar un registro |
| `GET` | `/api/v1/records/:id` | Obtener un registro por ID |
| `GET` | `/api/v1/records` | Listar registros con filtros |
| `POST` | `/api/v1/search` | Busqueda semantica (RAG sin LLM) |
| `POST` | `/api/v1/query` | Consulta en lenguaje natural (RAG + LLM) |
| `GET` | `/health` | Salud de la infraestructura (sin auth) |
| `GET` | `/health/live` | Probe de liveness |
| `GET` | `/health/ready` | Probe de readiness |
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

**List filters:** `entity_ref`, `actor_ref`, `record_type`, `source`, `tag`, `from`, `to`, `limit` (1-200), `cursor_time` + `cursor_id`.

---

## Data model

Everything that enters the engine is a **Record** — a universal, immutable, open-schema event:

```go
type Record struct {
<<<<<<< HEAD
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
=======
    RecordID   uuid.UUID      // identificador publico estable
    OccurredAt time.Time      // cuando ocurrio
    IngestedAt time.Time      // cuando se registro
    Source     string         // origen: "erp", "scanner", "manual"
    RecordType string         // tipo: "scan", "movement", "note"
    EntityRef  *string        // entidad: "lot:L-2024-001", "equipment:PUMP-01"
    ActorRef   *string        // actor: "user:maria@empresa.com"
    Title      *string        // titulo descriptivo
    Payload    map[string]any // datos libres (JSONB)
    ObjectRefs []string       // archivos adjuntos
    Tags       []string       // etiquetas
    Metadata   map[string]any // metadatos del sistema
    Checksum   string         // SHA-256 para deteccion de alteracion
}
```

**Inmutabilidad:** no existen operaciones de Update ni Delete. El almacen es append-only por diseno — reforzado a nivel de interfaz en Go, no por convencion.
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

---

## Architecture

```
internal/
<<<<<<< HEAD
  record/       ← Domain: universal model, validation, checksum
  indexing/     ← Domain: indexing pipeline (text → chunk → embed → store)
  query/        ← Domain: query engine (RAG + LLM)
  auth/         ← Domain: API key authentication
  platform/     ← Infrastructure adapters
=======
  record/       ← Dominio: modelo universal, validacion, checksum
  indexing/     ← Dominio: pipeline de indexacion (text → chunk → embed → store)
  query/        ← Dominio: motor de consulta (RAG + LLM)
  auth/         ← Dominio: autenticacion por API key
  platform/     ← Adaptadores de infraestructura
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)
    postgres/   ← RecordStore, IndexStore, APIKeyStore
    nats/       ← EventPublisher, Consumer
    ollama/     ← Embedder (local)
    anthropic/  ← LanguageModel (Claude)
    http/       ← Handlers, Router, Middleware
cmd/
<<<<<<< HEAD
  api/          ← HTTP server
  worker/       ← NATS consumer for async indexing
  toi/          ← CLI for ingestion and queries
```

**Hexagonal architecture.** Domain packages define ports (interfaces). Adapters implement them. Any infrastructure component can be replaced without touching the engine.
=======
  api/          ← Servidor HTTP
  worker/       ← Consumidor NATS para indexacion async
  toi/          ← CLI de ingesta y consulta
```

**Arquitectura hexagonal.** Los paquetes de dominio definen ports (interfaces). Los adaptadores los implementan. Se puede reemplazar cualquier componente de infraestructura sin tocar una linea del motor.
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

**Infrastructure:**

<<<<<<< HEAD
| Service | Purpose |
|---------|---------|
| PostgreSQL 16 + pgvector | Relational + vector + FTS storage |
| NATS JetStream | Async indexing pipeline |
| Ollama | Local embeddings (nomic-embed-text, 768 dim) |
=======
| Servicio | Funcion |
|----------|---------|
| PostgreSQL 16 + pgvector | Almacen relacional + vectorial + FTS |
| NATS JetStream | Pipeline async de indexacion |
| Ollama | Embeddings locales (nomic-embed-text, 768 dim) |

---

## Benchmarks

Ejecutar con `make bench`. Resultados en un i7-10700F:

| Benchmark | ops/seg | ns/op | allocs/op |
|-----------|---------|-------|-----------|
| Validate (minimo) | 186M | 6 ns | 0 |
| Checksum (50 keys) | 45K | 25 us | 169 |
| GenerateText (50 keys) | 62K | 19 us | 230 |
| Chunk (registro unico) | 14.9M | 87 ns | 1 |
| Pipeline completo (100 keys) | 29K | 40 us | 437 |
| AssembleContext (50 chunks) | 13K | 90 us | 419 |
| BuildPrompt (32KB) | 167K | 7.6 us | 1 |

La capa de dominio Go procesa ~25,000 registros/segundo. El cuello de botella real en produccion es Ollama (generacion de embeddings), no el motor en si.
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

---

## Benchmarks

<<<<<<< HEAD
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
=======
- **Embeddings:** locales. Los datos nunca salen de tu infraestructura para la generacion de vectores.
- **LLM:** externo y configurable. Vos elegis el proveedor (Claude, GPT, Llama, local). Solo las consultas viajan al LLM externo, no los registros completos.

Para privacidad total, implementa la interfaz `LanguageModel` contra un modelo local.
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

---

## Privacy

<<<<<<< HEAD
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
=======
El motor es generico. La implementacion es vertical.

| Vertical | Que registra | Quien lo usa |
|----------|-------------|--------------|
| **Trazabilidad operativa** | Escaneos, movimientos, notas de planta | Manufactura, logistica |
| **Compliance documental** | Evidencia regulatoria, certificaciones | Farmaceutica, alimentos |
| **Auditoria de campo** | Inspecciones, fotos, hallazgos | Construccion, energia |
| **Gestion de evidencia** | Documentos, cadena de custodia | Legal, gobierno |
| **Knowledge base operativa** | Procedimientos, lecciones aprendidas | Cualquier industria |
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

---

## Stack

<<<<<<< HEAD
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
=======
| Tecnologia | Version | Proposito |
|------------|---------|-----------|
| Go | 1.25 | Backend, CLI |
| PostgreSQL | 16 | Almacen 3-en-1 (relacional + pgvector + FTS) |
| pgvector | 0.7+ | Busqueda vectorial con HNSW |
| NATS | 2.10 | Mensajeria async (JetStream) |
| Ollama | latest | Embeddings locales |
| chi | v5 | Router HTTP |
| Docker Compose | — | Orquestacion del stack completo |

---

## Contribuir

TOI Motor es open source bajo licencia Apache 2.0. Las contribuciones son bienvenidas.

### Como contribuir

1. **Fork** del repositorio
2. **Crear un branch** para tu feature o fix (`git checkout -b feat/mi-feature`)
3. **Escribir tests** para la funcionalidad nueva
4. **Correr la suite de tests** antes de enviar (`make test && make lint`)
5. **Abrir un pull request** con una descripcion clara del que y el por que

### Areas donde se necesita ayuda

- **Adaptadores para nuevos LLMs** — implementar la interfaz `LanguageModel` para proveedores mas alla de Claude
- **Parsers de formatos** — extender el CLI para soportar mas formatos (Excel, Parquet, XML)
- **Modelos de embedding** — agregar soporte para proveedores alternativos de embeddings
- **Soporte de idiomas** — mejorar la extraccion de entidades para distintos idiomas
- **Documentacion** — tutoriales, ejemplos para verticales especificas, guias de despliegue
- **Tests de integracion** — expandir la cobertura E2E con escenarios realistas
- **Performance** — benchmarks y optimizaciones para ingesta de alto throughput

### Setup de desarrollo
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

```bash
git clone https://github.com/ricaguroc/toi-motor.git
cd toi-motor
<<<<<<< HEAD
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
=======
make dev-infra       # levantar postgres, nats, ollama
make test            # correr tests
make bench           # correr benchmarks
make dev-api         # correr API localmente
make dev-worker      # correr worker localmente (terminal separada)
```

### Convenciones del codigo

- Arquitectura hexagonal: los paquetes de dominio (`internal/record`, `internal/query`, `internal/indexing`) definen interfaces; los adaptadores (`internal/platform/*`) las implementan
- Los tests van junto al codigo que testean (`*_test.go` en el mismo paquete)
- No hay `Update` ni `Delete` sobre records — append-only por diseno
- Todos los endpoints de API bajo `/api/v1/*` requieren `X-API-Key`

---

## Documentacion

- [Whitepaper tecnico](docs/whitepaper.md) — arquitectura, modelo de datos, pipeline completo
- [Motor vs Adapters](docs/paper-motor-vs-adapters.md) — separacion arquitectonica y optimizaciones
- [Release tecnico v2](docs/release-v2.md) — estado actual, API implementada, resultados E2E
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

---

## License

Apache 2.0 — [see LICENSE](LICENSE)

<<<<<<< HEAD
No vendor lock-in. No black boxes. Auditable code.
=======
Sin vendor lock-in. Sin cajas negras. Codigo auditable.
>>>>>>> c4fc7e5 (docs: rewrite README with make commands, curl examples, and contributing guide)

---

> *Reality, when correctly recorded, can be queried as if it were a system.*
