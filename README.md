# TOI — Motor Universal de Registros

Ingesta cualquier dato. Consultá en lenguaje natural.

TOI es un motor que transforma datos heterogéneos en conocimiento buscable usando embeddings, RAG y LLM. Open source, API-first, agnóstico al proveedor de LLM.

```bash
$ toi ingest file facturas.csv --source erp --type factura
✓ 1,847 registros ingestados en 2.3s

$ toi query "¿qué envíos se retrasaron la semana pasada?"
Se encontraron 12 registros en 3 fuentes...

1. Envío #LP-4821 — retrasado 2 días
   Fuente: ERP · Registrado: 2026-03-12
```

---

## Cómo funciona

```
┌─────────────────────────────────────────────────────────────────┐
│                         FUENTES                                  │
│  CSV · ERP · Escáners · Formularios · Email · APIs · Sensores   │
└────────────────────────────┬──────────────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │    CLI / API     │  toi ingest file, POST /records
                    └────────┬────────┘
                             │
              ┌──────────────▼──────────────┐
              │    Motor Universal (TOI)     │
              │                              │
              │  01 INGESTA                  │
              │  Registro universal flexible │
              │  Append-only, inmutable      │
              │  Checksum SHA-256            │
              │                              │
              │  02 INDEXACIÓN               │
              │  Chunking + Embeddings       │
              │  Pipeline async via NATS     │
              │  pgvector con HNSW           │
              │                              │
              │  03 CONSULTA                 │
              │  RAG + LLM agnóstico         │
              │  Lenguaje natural            │
              │  Respuestas con evidencia    │
              └──────────────────────────────┘
```

## Inicio rápido

### Prerrequisitos

- Go 1.22+
- Docker y Docker Compose

### 1. Clonar y levantar

```bash
git clone https://github.com/trazabilidad-io/motor.git
cd motor
docker compose up -d
```

Esto levanta PostgreSQL (pgvector), NATS JetStream, Ollama, MinIO y Redis.

### 2. Ejecutar las migraciones y crear una API key

```bash
go run ./cmd/api/    # levanta el servidor HTTP en :8080
go run ./cmd/worker/ # levanta el worker de indexación
```

```bash
# En otra terminal: crear una API key
go run ./scripts/create-apikey.go
# Output: tu-api-key-aquí
```

### 3. Ingestar registros

**Via CLI:**

```bash
export TOI_API_KEY=tu-api-key-aquí

# Importar un CSV
toi ingest file datos.csv --source erp --type movimiento

# Importar JSON
toi ingest file registros.json --source scanner --type escaneo

# Importar JSONL (un registro por línea)
toi ingest bulk historico.jsonl

# Monitorear una carpeta (auto-ingesta archivos nuevos)
toi watch /data/escaneos --source scanner --type escaneo
```

**Via API:**

```bash
curl -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: tu-api-key-aquí" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "scanner_dock_3",
    "record_type": "scan",
    "entity_ref": "lot:LP-4821",
    "actor_ref": "operator:jperez",
    "title": "Escaneo de ingreso lote LP-4821",
    "payload": {
      "temperature_c": 4.2,
      "weight_kg": 1250.5,
      "seal_intact": true
    },
    "tags": ["cold-chain", "incoming"]
  }'
```

### 4. Consultar

```bash
# Consulta en lenguaje natural (RAG + LLM)
toi query "¿qué pasó con el lote LP-4821?"

# Búsqueda semántica (solo RAG, sin LLM)
toi search "lote LP-4821 temperatura"
```

---

## CLI

El CLI `toi` es la capa de conveniencia sobre la API. Compila como un solo binario Go.

```bash
go build -o toi ./cmd/toi/
```

| Comando | Descripción |
|---------|-------------|
| `toi ingest file <path>` | Importa CSV o JSON como registros |
| `toi ingest bulk <path>` | Importa JSONL (un registro por línea) |
| `toi query <pregunta>` | Consulta en lenguaje natural (RAG + LLM) |
| `toi search <query>` | Búsqueda semántica sin LLM |
| `toi watch <dir>` | Monitorea carpeta y auto-ingesta archivos nuevos |

**Configuración:**

| Flag | Variable de entorno | Default |
|------|-------------------|---------|
| `--api-url` | `TOI_API_URL` | `http://localhost:8080` |
| `--api-key` | `TOI_API_KEY` | (requerido) |
| `--source` | — | `cli` |
| `--type` | — | `document` |

**Mapeo CSV:** las columnas `source`, `record_type`, `occurred_at`, `entity_ref`, `actor_ref`, `title` y `tags` se mapean a campos del registro. Cualquier otra columna va a `payload`.

---

## API

Todos los endpoints bajo `/api/v1/*` requieren header `X-API-Key`.

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| `POST` | `/api/v1/records` | Ingestar un registro |
| `GET` | `/api/v1/records/:id` | Obtener un registro por ID |
| `GET` | `/api/v1/records` | Listar registros con filtros |
| `POST` | `/api/v1/search` | Búsqueda semántica (RAG sin LLM) |
| `POST` | `/api/v1/query` | Consulta en lenguaje natural (RAG + LLM) |
| `GET` | `/health` | Estado de la infraestructura |

**Filtros de listado:** `entity_ref`, `actor_ref`, `record_type`, `source`, `tag`, `from`, `to`, `limit` (1-200), `cursor_time` + `cursor_id`.

---

## Modelo de datos

Todo lo que entra al motor es un **Record** — un registro universal, inmutable y con esquema abierto:

```go
type Record struct {
    RecordID   uuid.UUID      // identificador público estable
    OccurredAt time.Time      // cuándo ocurrió
    IngestedAt time.Time      // cuándo se registró
    Source     string         // origen: "erp", "scanner", "manual"
    RecordType string         // tipo: "scan", "movement", "note"
    EntityRef  *string        // entidad: "lot:L-2024-001", "equipment:PUMP-01"
    ActorRef   *string        // actor: "user:maria@empresa.com"
    Title      *string        // título descriptivo
    Payload    map[string]any // datos libres (JSONB)
    ObjectRefs []string       // archivos adjuntos
    Tags       []string       // etiquetas
    Metadata   map[string]any // metadatos del sistema
    Checksum   string         // SHA-256 para detección de alteración
}
```

**Inmutabilidad:** no existen operaciones de Update ni Delete. El almacén es append-only por diseño — reforzado a nivel de interfaz en Go, no por convención.

---

## Arquitectura

```
internal/
  record/       ← Dominio: modelo universal, validación, checksum
  indexing/     ← Dominio: pipeline de indexación (text → chunk → embed → store)
  query/        ← Dominio: motor de consulta (RAG + LLM)
  auth/         ← Dominio: autenticación por API key
  platform/     ← Adaptadores de infraestructura
    postgres/   ← RecordStore, IndexStore, APIKeyStore
    nats/       ← EventPublisher, Consumer
    ollama/     ← Embedder (local)
    anthropic/  ← LanguageModel (Claude)
    http/       ← Handlers, Router, Middleware

cmd/
  api/          ← Servidor HTTP
  worker/       ← Consumidor NATS para indexación async
  toi/          ← CLI de ingesta y consulta
```

**Arquitectura hexagonal.** Los paquetes de dominio definen ports (interfaces). Los adaptadores los implementan. Se puede reemplazar cualquier componente de infraestructura sin tocar una línea del motor.

**Infraestructura:**

| Servicio | Función |
|----------|---------|
| PostgreSQL 16 + pgvector | Almacén relacional + vectorial + FTS |
| NATS JetStream | Pipeline async de indexación |
| Ollama | Embeddings locales (nomic-embed-text, 768 dim) |
| MinIO | Almacenamiento de objetos |
| Redis | Cache (reservado) |

---

## Privacidad

- **Embeddings:** locales. Los datos nunca salen de tu infraestructura para la generación de vectores.
- **LLM:** externo y configurable. El usuario elige el proveedor (Claude, GPT, Llama, local). Solo las consultas viajan al LLM externo, no los registros completos.

Si se requiere privacidad total, se implementa la interfaz `LanguageModel` contra un modelo local.

---

## Casos de uso

El motor es genérico. La implementación es vertical.

| Vertical | Qué registra | Quién lo usa |
|----------|-------------|--------------|
| **Trazabilidad operativa** | Escaneos, movimientos, notas de planta | Manufactura, logística |
| **Compliance documental** | Evidencia regulatoria, certificaciones | Farmacéutica, alimentos |
| **Auditoría de campo** | Inspecciones, fotos, hallazgos | Construcción, energía |
| **Gestión de evidencia** | Documentos, cadena de custodia | Legal, gobierno |
| **Knowledge base operativa** | Procedimientos, lecciones aprendidas | Cualquier industria |

---

## Stack

| Tecnología | Versión | Propósito |
|------------|---------|-----------|
| Go | 1.22+ | Backend, CLI |
| PostgreSQL | 16 | Almacén 3-en-1 (relacional + pgvector + FTS) |
| pgvector | 0.7+ | Búsqueda vectorial con HNSW |
| NATS | 2.10 | Mensajería async (JetStream) |
| Ollama | latest | Embeddings locales |
| chi | v5 | Router HTTP |
| Docker Compose | — | Orquestación de infraestructura |

---

## Métricas

| Métrica | Valor |
|---------|-------|
| Archivos Go | 69 |
| Tests | 119 |
| Binarios | 3 (api, worker, toi) |
| Paquetes de dominio | 4 |
| Paquetes de adaptador | 6 |
| Latencia ingesta | ~5-10ms |
| Latencia búsqueda | ~50-200ms |
| Latencia query (LLM) | ~2-5s |

---

## Documentación

- [Whitepaper técnico](docs/whitepaper.md) — arquitectura, modelo de datos, pipeline completo
- [Motor vs Adapters](docs/paper-motor-vs-adapters.md) — separación arquitectónica y optimizaciones
- [Release técnico v2](docs/release-v2.md) — estado actual, API implementada, resultados E2E

---

## Licencia

Apache 2.0 — [ver LICENSE](LICENSE)

Sin vendor lock-in. Sin cajas negras. Código auditable.

---

> *La realidad, cuando es correctamente registrada, puede ser consultada como si fuera un sistema.*
