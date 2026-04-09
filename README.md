# TOI Motor — Motor Universal de Registros

Ingesta cualquier dato. Consulta en lenguaje natural.

TOI es un motor que transforma datos heterogeneos en conocimiento buscable usando embeddings, RAG y LLMs. Open source, API-first, agnostico al proveedor de LLM.

```bash
$ toi ingest file facturas.csv --source erp --type factura
✓ 1,847 registros ingestados en 2.3s

$ toi query "¿que envios se retrasaron la semana pasada?"
Se encontraron 12 registros en 3 fuentes...

1. Envio #LP-4821 — retrasado 2 dias
   Fuente: ERP · Registrado: 2026-03-12
```

---

## Como funciona

```
┌──────────────────────────────────────────────────────────────┐
│                         FUENTES                              │
│  CSV · ERP · Escaners · Formularios · Email · APIs · Sensores│
└────────────────────────────┬─────────────────────────────────┘
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

### Prerrequisitos

- Docker y Docker Compose

Nada mas. Todo corre containerizado.

### 1. Clonar y desplegar

```bash
git clone https://github.com/ricaguroc/toi-motor.git
cd toi-motor
make deploy
```

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

```bash
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "warehouse",
    "record_type": "scan",
    "entity_ref": "lot:LP-4821",
    "actor_ref": "operator:jperez",
    "title": "Escaneo de ingreso lote LP-4821",
    "payload": {
      "temperature_c": 4.2,
      "weight_kg": 1250.5,
      "seal_intact": true,
      "dock": "D3"
    },
    "tags": ["cold-chain", "incoming"]
  }'
```

Respuesta:

```json
{
  "record_id": "a1b2c3d4-...",
  "ingested_at": "2026-04-09T10:30:00Z",
  "checksum": "sha256:ab3f..."
}
```

#### Ingestar mas registros (construir contexto)

```bash
# Alerta de temperatura en el mismo lote
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "sensor_dock_3",
    "record_type": "alert",
    "entity_ref": "lot:LP-4821",
    "title": "Excursion de temperatura detectada",
    "payload": {
      "temperature_c": 9.8,
      "threshold_c": 8.0,
      "duration_min": 14,
      "zone": "cold-storage-A"
    },
    "tags": ["cold-chain", "alert", "temperature"]
  }'

# Movimiento a retencion de calidad
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "wms",
    "record_type": "movement",
    "entity_ref": "lot:LP-4821",
    "actor_ref": "operator:mgarcia",
    "title": "Lote movido a retencion de calidad",
    "payload": {
      "from_location": "cold-storage-A",
      "to_location": "quality-hold-1",
      "reason": "excursion de temperatura pendiente de revision"
    },
    "tags": ["quality", "hold"]
  }'

# Otro lote — operacion normal
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "warehouse",
    "record_type": "scan",
    "entity_ref": "lot:LP-5010",
    "actor_ref": "operator:jperez",
    "title": "Escaneo de ingreso lote LP-5010",
    "payload": {
      "temperature_c": 3.1,
      "weight_kg": 980.0,
      "seal_intact": true,
      "dock": "D1"
    },
    "tags": ["cold-chain", "incoming"]
  }'

# Registro de mantenimiento de equipo
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "maintenance",
    "record_type": "work_order",
    "entity_ref": "equipment:COMP-A01",
    "actor_ref": "tech:rlopez",
    "title": "Compresor A01 — mantenimiento programado",
    "payload": {
      "work_type": "preventive",
      "findings": "Refrigerante bajo, recargado a especificacion. Filtro reemplazado.",
      "downtime_min": 45,
      "parts_used": ["filter-FK200", "refrigerant-R404A"]
    },
    "tags": ["maintenance", "cold-chain", "compressor"]
  }'
```

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
curl -s "http://localhost:8080/api/v1/records?source=maintenance" \
  -H "X-API-Key: YOUR_API_KEY" | jq
```

#### Busqueda semantica (RAG, sin LLM)

```bash
curl -s -X POST http://localhost:8080/api/v1/search \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"q": "problemas de temperatura en cold storage"}' | jq
```

#### Consulta en lenguaje natural (RAG + LLM)

Requiere `LLM_API_KEY` configurada en `docker-compose.yml`.

```bash
curl -s -X POST http://localhost:8080/api/v1/query \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "q": "¿Que paso con el lote LP-4821?",
    "format": "conversational"
  }' | jq
```

Respuesta:

```json
{
  "answer": "El lote LP-4821 llego al muelle D3 con una temperatura de 4.2°C. Un sensor detecto posteriormente una excursion de temperatura a 9.8°C (umbral: 8.0°C) durante 14 minutos en cold-storage-A. El lote fue movido a quality-hold-1 pendiente de revision.",
  "confidence": "high",
  "records_cited": ["a1b2c3d4-...", "e5f6g7h8-...", "i9j0k1l2-..."],
  "gaps": null,
  "suggested_followup": [
    "¿Se resolvio la excursion de temperatura?",
    "¿Que otros lotes estaban en cold-storage-A durante ese periodo?"
  ],
  "retrieved_count": 3,
  "query_ms": 2340
}
```

#### Verificar salud del sistema

```bash
curl -s http://localhost:8080/health | jq
```

---

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
```

---

## CLI

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
|------|---------------------|---------|
| `--api-url` | `TOI_API_URL` | `http://localhost:8080` |
| `--api-key` | `TOI_API_KEY` | (requerido) |
| `--source` | — | `cli` |
| `--type` | — | `document` |

**Mapeo CSV:** las columnas `source`, `record_type`, `occurred_at`, `entity_ref`, `actor_ref`, `title` y `tags` se mapean a campos del registro. Cualquier otra columna va a `payload`.

---

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

**Filtros de listado:** `entity_ref`, `actor_ref`, `record_type`, `source`, `tag`, `from`, `to`, `limit` (1-200), `cursor_time` + `cursor_id`.

---

## Modelo de datos

Todo lo que entra al motor es un **Record** — un registro universal, inmutable y con esquema abierto:

```go
type Record struct {
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

---

## Arquitectura

```
internal/
  record/       ← Dominio: modelo universal, validacion, checksum
  indexing/     ← Dominio: pipeline de indexacion (text → chunk → embed → store)
  query/        ← Dominio: motor de consulta (RAG + LLM)
  auth/         ← Dominio: autenticacion por API key
  platform/     ← Adaptadores de infraestructura
    postgres/   ← RecordStore, IndexStore, APIKeyStore
    nats/       ← EventPublisher, Consumer
    ollama/     ← Embedder (local)
    anthropic/  ← LanguageModel (Claude)
    http/       ← Handlers, Router, Middleware
cmd/
  api/          ← Servidor HTTP
  worker/       ← Consumidor NATS para indexacion async
  toi/          ← CLI de ingesta y consulta
```

**Arquitectura hexagonal.** Los paquetes de dominio definen ports (interfaces). Los adaptadores los implementan. Se puede reemplazar cualquier componente de infraestructura sin tocar una linea del motor.

**Infraestructura:**

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

---

## Privacidad

- **Embeddings:** locales. Los datos nunca salen de tu infraestructura para la generacion de vectores.
- **LLM:** externo y configurable. Vos elegis el proveedor (Claude, GPT, Llama, local). Solo las consultas viajan al LLM externo, no los registros completos.

Para privacidad total, implementa la interfaz `LanguageModel` contra un modelo local.

---

## Casos de uso

El motor es generico. La implementacion es vertical.

| Vertical | Que registra | Quien lo usa |
|----------|-------------|--------------|
| **Trazabilidad operativa** | Escaneos, movimientos, notas de planta | Manufactura, logistica |
| **Compliance documental** | Evidencia regulatoria, certificaciones | Farmaceutica, alimentos |
| **Auditoria de campo** | Inspecciones, fotos, hallazgos | Construccion, energia |
| **Gestion de evidencia** | Documentos, cadena de custodia | Legal, gobierno |
| **Knowledge base operativa** | Procedimientos, lecciones aprendidas | Cualquier industria |

---

## Stack

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

```bash
git clone https://github.com/ricaguroc/toi-motor.git
cd toi-motor
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

---

## Licencia

Apache 2.0 — [ver LICENSE](LICENSE)

Sin vendor lock-in. Sin cajas negras. Codigo auditable.

---

> *La realidad, cuando es correctamente registrada, puede ser consultada como si fuera un sistema.*
