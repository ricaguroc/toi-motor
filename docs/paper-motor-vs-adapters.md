# Motor vs. Adapters: Separacion Arquitectonica y Optimizacion del Motor de Trazabilidad Operativa Inteligente

**Version:** 1.0  
**Fecha:** 2026-04-07  
**Autor:** Ricardo Gururoc  
**Tipo:** Paper de investigacion interna  

---

## Resumen

Este documento establece la separacion formal entre el **Motor** (nucleo de dominio) y los **Adapters** (conectores con la realidad) del sistema de Trazabilidad Operativa Inteligente. El objetivo es triple: (1) definir los limites arquitectonicos con precision, (2) demostrar que la realidad es agnostica al motor — cualquier mejora interna no altera como el mundo interactua con el sistema, y (3) identificar optimizaciones concretas en velocidad, reduccion de fallas y eficiencia de tokens.

---

## 1. Definicion de los limites

### 1.1 El Motor

El Motor es la logica de dominio pura. No tiene dependencias externas. No sabe que existe PostgreSQL, NATS, Ollama, ni HTTP. Solo conoce interfaces (ports) que definen contratos abstractos.

```
internal/
  record/       ← Dominio: modelo universal de registro
  indexing/      ← Dominio: pipeline de indexacion y busqueda semantica
  query/         ← Dominio: motor de consulta (RAG + LLM)
  auth/          ← Dominio: autenticacion por API key
```

**Responsabilidades del Motor:**

| Componente | Responsabilidad | Depende de |
|------------|----------------|------------|
| `record.Record` | Modelo universal de registro. Inmutable, append-only, con checksum SHA-256 | Nada |
| `record.RecordService` | Validacion, enriquecimiento, ingesta de registros | `RecordStore` (port), `EventPublisher` (port, opcional) |
| `indexing.Pipeline` | Chunking, embedding, almacenamiento vectorial | `RecordStore` (port), `Chunker` (port), `Embedder` (port), `IndexStore` (port) |
| `indexing.Chunk` | Representacion de fragmento de texto para embedding | Nada |
| `query.Retriever` | Busqueda semantica: pregunta → chunks relevantes | `Embedder` (port), `IndexStore` (port) |
| `query.QueryService` | RAG completo: pregunta → contexto → LLM → respuesta | `Retriever`, `LanguageModel` (port) |
| `auth.APIKey` | Modelo de API key con hash, expiracion, revocacion | Nada |

### 1.2 Los Ports (contratos)

Los ports son interfaces Go que definen QUE necesita el motor, sin decir COMO se satisface. Son el contrato entre el motor y la realidad.

```go
// === ALMACENAMIENTO DE REGISTROS ===
// Port: record.RecordStore
type RecordStore interface {
    Append(ctx context.Context, r Record) error
    GetByID(ctx context.Context, recordID uuid.UUID) (Record, error)
    List(ctx context.Context, filter Filter) (ListResult, error)
}
// Nota: NO hay Update ni Delete. Inmutabilidad por diseno.

// === PUBLICACION DE EVENTOS ===
// Port: record.EventPublisher
type EventPublisher interface {
    Publish(ctx context.Context, subject string, data []byte) error
}

// === ALMACENAMIENTO VECTORIAL ===
// Port: indexing.IndexStore
type IndexStore interface {
    UpsertChunks(ctx context.Context, chunks []Chunk, embeddings [][]float32) error
    SearchSimilar(ctx context.Context, queryEmbedding []float32, filter SearchFilter, limit int) ([]SearchResult, error)
}

// === GENERACION DE EMBEDDINGS ===
// Port: indexing.Embedder
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int
}

// === MODELO DE LENGUAJE ===
// Port: query.LanguageModel
type LanguageModel interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// === AUTENTICACION ===
// Port: auth.APIKeyStore
type APIKeyStore interface {
    Validate(ctx context.Context, rawKey string) (*APIKey, error)
    UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}
```

### 1.3 Los Adapters

Los Adapters implementan los ports. Son el puente entre el motor y la infraestructura real. Viven en `internal/platform/` y cada uno envuelve UNA dependencia externa.

```
internal/platform/
  postgres/     ← Implementa RecordStore, IndexStore, APIKeyStore
  nats/         ← Implementa EventPublisher + consumer loop
  ollama/       ← Implementa Embedder + LanguageModel (local)
  anthropic/    ← Implementa LanguageModel (externo)
  http/         ← Expone el motor via REST API
  minio/        ← (stub) Implementara almacenamiento de objetos
  redis/        ← (stub) Implementara cache de consultas
```

**Mapa de adapter a port:**

| Adapter | Port que implementa | Dependencia externa |
|---------|--------------------|--------------------|
| `postgres.PostgresRecordStore` | `record.RecordStore` | PostgreSQL 16 (pgx) |
| `postgres.PostgresEmbeddingRepo` | `indexing.IndexStore` | PostgreSQL 16 + pgvector |
| `postgres.PostgresAPIKeyStore` | `auth.APIKeyStore` | PostgreSQL 16 (bcrypt) |
| `nats.Publisher` | `record.EventPublisher` | NATS JetStream |
| `ollama.OllamaEmbedder` | `indexing.Embedder` | Ollama (local HTTP) |
| `ollama.OllamaLLMClient` | `query.LanguageModel` | Ollama (local HTTP) |
| `anthropic.Client` | `query.LanguageModel` | Anthropic API (externo) |
| `http.Router` | — (expone, no implementa) | chi/v5 |

### 1.4 Diagrama de capas

```
                    LA REALIDAD
                        |
    ┌───────────────────┼───────────────────┐
    |           ADAPTERS (platform/)        |
    |                                       |
    |  WhatsApp   HTTP    Email   Telegram  |  ← Entrada (futuros)
    |  Ollama    Anthropic  OpenAI  Local   |  ← LLM (intercambiables)
    |  PostgreSQL  SQLite  DynamoDB         |  ← Storage (intercambiables)
    |  NATS      Kafka    RabbitMQ          |  ← Eventos (intercambiables)
    |                                       |
    ├───────────────────────────────────────┤
    |              PORTS (interfaces)       |
    |                                       |
    |  RecordStore    EventPublisher        |
    |  IndexStore     Embedder              |
    |  LanguageModel  APIKeyStore           |
    |                                       |
    ├───────────────────────────────────────┤
    |              MOTOR (dominio)          |
    |                                       |
    |  Record Model   RecordService         |
    |  Pipeline        Chunker              |
    |  Retriever      QueryService          |
    |                                       |
    └───────────────────────────────────────┘
```

**Propiedad fundamental:** se puede reemplazar CUALQUIER adapter sin tocar una sola linea del motor. Se puede reescribir el motor entero sin que ningun adapter cambie su interfaz publica.

---

## 2. El Motor en detalle

### 2.1 Universal Record Model

El registro universal es la unidad atomica del sistema. Todo lo que ocurre en la operacion se representa como un `Record`:

```go
type Record struct {
    RecordID   uuid.UUID              // Identidad unica global
    OccurredAt time.Time              // Cuando ocurrio en la realidad
    IngestedAt time.Time              // Cuando entro al sistema
    Source     string                  // Origen: "scanner_app", "whatsapp", "erp"
    RecordType string                 // Tipo: scan, movement, note, photo, etc.
    EntityRef  *string                // Entidad: "lot:L-2024-001", "equipment:PUMP-01"
    ActorRef   *string                // Quien: "user:juan@empresa.com"
    Title      *string                // Titulo legible
    Payload    map[string]interface{} // Datos libres (schema-free)
    ObjectRefs []string               // Referencias a archivos (MinIO)
    Tags       []string               // Etiquetas para filtrado
    Metadata   map[string]interface{} // Metadata del sistema
    Checksum   string                 // SHA-256 del contenido canonico
}
```

**Propiedades invariantes:**

1. **Inmutabilidad:** No existe operacion de Update ni Delete en el port `RecordStore`. Un registro, una vez creado, es permanente. Esta inmutabilidad esta garantizada a nivel de interfaz — no es una convencion, es una restriccion de tipo.

2. **Integridad:** Cada registro lleva un checksum SHA-256 calculado sobre los campos canonicos (occurred_at + source + record_type + entity_ref + actor_ref + payload). Cualquier alteracion posterior es detectable.

3. **Universalidad:** El campo `Payload` es schema-free (`map[string]interface{}`). Esto permite que el mismo modelo represente un escaneo de codigo de barras, una nota de voz transcrita, un reporte de incidente, o una lectura de sensor de temperatura. El motor no necesita conocer el schema — eso es responsabilidad de la capa de configuracion vertical.

### 2.2 Pipeline de indexacion

El pipeline transforma registros crudos en vectores buscables:

```
Record → Chunker → []Chunk → Embedder → [][]float32 → IndexStore
```

**Estrategias de chunking disponibles:**

| Estrategia | Comportamiento | Uso ideal |
|------------|---------------|-----------|
| `NoOpChunker` | Un chunk = un registro completo | Registros cortos (notas, escaneos) |
| `ParagraphChunker` | Divide por parrafos (\n\n) | Documentos con estructura |
| `SlidingWindowChunker` | Ventana deslizante con overlap | Textos largos sin estructura |
| `DefaultChunker` | Alias de `NoOpChunker` | Default del worker |

**Flujo asincrono:**

```
API (Append) → NATS "record.ingested" → Worker → Pipeline → PostgreSQL+pgvector
```

El worker consume eventos de NATS con retry automatico (max 10 intentos, 60s ack timeout). Los errores permanentes (`indexing.ErrPermanent`) se descartan para evitar loops infinitos.

### 2.3 Motor de consulta (RAG)

El motor de consulta opera en dos modos:

**Modo Search (sin LLM):**
```
Pregunta → Embedder → vector → IndexStore.SearchSimilar → []SearchResult
```
Latencia: 100-200ms. Determinista. Sin alucinaciones.

**Modo Query (con LLM):**
```
Pregunta → Retriever.Retrieve → contexto → LanguageModel.Complete → respuesta NL
```

El `QueryService` construye un prompt con los chunks recuperados como contexto y pide al LLM que responda en formato JSON estructurado:

```go
type QueryResponse struct {
    Answer            string   // Respuesta en lenguaje natural
    Confidence        string   // "high", "medium", "low"
    RecordsCited      []string // IDs de registros usados como evidencia
    Gaps              *string  // Informacion que falta para responder mejor
    SuggestedFollowup []string // Preguntas de seguimiento sugeridas
    RetrievedCount    int      // Chunks recuperados
    QueryMs           int64    // Latencia total en milisegundos
}
```

**Extraccion de entidades:** El motor extrae entity_refs de preguntas en lenguaje natural usando 8 patrones regex:

```
"lote L-2024-001"     → "lot:L-2024-001"
"equipo PUMP-01"      → "equipment:PUMP-01"
"vehiculo TRK-103"    → "vehicle:TRK-103"
"pedido PO-2024-001"  → "order:PO-2024-001"
"usuario juan@x.com"  → "user:juan@x.com"
```

Esto permite filtrar los embeddings antes de la busqueda vectorial, reduciendo el espacio de busqueda y mejorando precision.

---

## 3. Los Adapters en detalle

### 3.1 Taxonomia de adapters

Los adapters se clasifican en cuatro categorias segun su funcion:

```
ADAPTERS
  |
  ├── INGESTA (como entran datos al motor)
  |     ├── HTTP REST API (actual)
  |     ├── WhatsApp Business API (propuesto)
  |     ├── Email forwarding (propuesto)
  |     ├── Telegram Bot (propuesto)
  |     └── SDK movil (propuesto)
  |
  ├── PERSISTENCIA (donde se almacenan datos)
  |     ├── PostgreSQL (actual: records + embeddings + auth)
  |     ├── MinIO (stub: almacenamiento de archivos)
  |     └── Redis (stub: cache de consultas)
  |
  ├── PROCESAMIENTO (como se transforman datos)
  |     ├── Ollama Embedder (actual: embeddings locales)
  |     ├── Ollama LLM (actual: consulta local)
  |     ├── Anthropic LLM (actual: consulta externa)
  |     └── Whisper (propuesto: transcripcion de audio)
  |
  └── TRANSPORTE (como se mueven datos internamente)
        └── NATS JetStream (actual: eventos asinc.)
```

### 3.2 Adapter actual: PostgreSQL

PostgreSQL implementa 3 ports simultaneamente, lo cual es una decision pragmatica — un solo servicio de infraestructura cubre almacenamiento relacional, vectorial y de autenticacion:

- `RecordStore` → tabla `records` (14 columnas, 10 indices, FTS con tokenizer espanol)
- `IndexStore` → tabla `record_embeddings` (vector(768), indice HNSW con m=16, ef_construction=64)
- `APIKeyStore` → tabla `api_keys` (hash bcrypt, expiracion, revocacion)

**Denormalizacion intencional:** `record_embeddings` duplica campos de `records` (entity_ref, actor_ref, record_type, occurred_at) para permitir filtrado pre-busqueda sin JOIN. Esto sacrifica normalizacion por velocidad en queries vectoriales.

### 3.3 Adapter actual: NATS JetStream

NATS actua como bus de eventos asincrono entre la API y el worker de indexacion:

- Stream `RECORDS` con subjects `record.ingested` y `record.indexed`
- Retencion: 7 dias, storage en disco
- Consumer durable con ack explicito
- Retry con backoff: max 10 reintentos, 60s timeout por mensaje

**Propiedad critica:** NATS es OPCIONAL. Si no esta disponible, el `RecordService` simplemente no publica eventos. Los registros se almacenan pero no se indexan. Esto permite degradacion graceful.

### 3.4 Adapter actual: Ollama

Ollama corre localmente y provee dos servicios:

- **Embeddings** (port `Embedder`): modelo `nomic-embed-text`, dimensiones 768, timeout 30s
- **LLM** (port `LanguageModel`): modelo configurable, timeout 120s, output JSON

**Propiedad de privacidad:** Los datos operativos nunca salen de la infraestructura local para embedding. Solo las consultas al LLM externo (Anthropic) cruzan el boundary de red.

### 3.5 Adapter actual: Anthropic

Implementa el port `LanguageModel` contra la API de Anthropic. Es intercambiable con el LLM de Ollama — el motor no distingue cual esta respondiendo. Configuracion:

- `max_tokens: 4096` (fijo)
- `anthropic-version: 2023-06-01`
- Concatena todos los content blocks de la respuesta

### 3.6 Adapter actual: HTTP

El adapter HTTP NO implementa un port — ES la capa de exposicion. Traduce HTTP a llamadas de dominio:

```
POST /api/v1/records     → RecordService.Ingest()
GET  /api/v1/records/:id → RecordStore.GetByID()
GET  /api/v1/records     → RecordStore.List()
POST /api/v1/query       → QueryService.Query()
POST /api/v1/search      → Retriever.Retrieve()
GET  /health/*           → HealthChecker (infra)
```

Middleware stack: RequestID → RealIP → Recoverer → RequestLogger → APIKeyMiddleware.

---

## 4. Degradacion graceful — tabla de modos

El motor opera en modos progresivos segun la disponibilidad de adapters:

| Modo | Adapters requeridos | Funcionalidad |
|------|-------------------|---------------|
| **Ledger** | PostgreSQL | Solo ingesta y lectura de registros. Append-only ledger puro. Sin busqueda. |
| **Search** | PostgreSQL + NATS + Ollama | Ingesta + indexacion asincrona + busqueda semantica. Sin LLM. |
| **Full** | PostgreSQL + NATS + Ollama + LLM | Ingesta + indexacion + busqueda + consulta en lenguaje natural. |

Esta degradacion NO requiere configuracion. El motor detecta que adapters estan disponibles en startup y habilita/deshabilita endpoints automaticamente. Un endpoint no disponible retorna HTTP 503 con mensaje explicativo.

---

## 5. Optimizaciones propuestas para el Motor

### 5.1 Velocidad

#### 5.1.1 API Key validation: de O(n) a O(1)

**Estado actual:** `PostgresAPIKeyStore.Validate()` itera TODAS las API keys activas y compara con bcrypt una por una. Con 100 keys, cada request paga 100 comparaciones bcrypt (~100ms).

**Propuesta:** Agregar un cache en memoria con key_prefix como lookup rapido. Bcrypt solo se ejecuta contra el candidato que matchea el prefijo.

```
Actual:   Request → iterate all keys → bcrypt each → match
Propuesto: Request → prefix lookup (O(1)) → bcrypt 1 key → match
```

**Impacto estimado:** Reduccion de latencia de autenticacion de ~O(n*100ms) a ~O(100ms) constante.

#### 5.1.2 Embedding batch: de secuencial a pipeline

**Estado actual:** El pipeline de indexacion procesa un registro a la vez: chunk → embed → store. Si hay 1000 registros pendientes, cada uno espera al anterior.

**Propuesta:** Implementar batch processing en el worker:

```
Actual:   msg₁ → chunk → embed → store → msg₂ → chunk → embed → store
Propuesto: [msg₁..msgN] → chunk all → embed batch → store batch (transaccion)
```

Ollama ya soporta batch embedding (multiples textos en un request). El adapter ya envia `[]string`. La optimizacion esta en el worker, no en el adapter.

**Impacto estimado:** Reduccion de llamadas HTTP al embedder de N a 1 por batch. Reduccion de transacciones PostgreSQL de N a 1.

#### 5.1.3 HNSW index tuning

**Estado actual:** `m = 16, ef_construction = 64`. Estos son valores conservadores.

**Propuestas segun volumen:**

| Registros | m | ef_construction | ef_search | Tradeoff |
|-----------|---|----------------|-----------|----------|
| < 10K | 16 | 64 | 40 | Actual: equilibrado |
| 10K-100K | 16 | 128 | 80 | Mayor precision, build mas lento |
| > 100K | 24 | 200 | 100 | Maxima precision, mas memoria |

**Nota:** `ef_search` se configura por sesion (`SET hnsw.ef_search = N`), no requiere rebuild del indice.

#### 5.1.4 Connection pooling del HTTP client

**Estado actual:** Los adapters Ollama y Anthropic crean `http.Client` con timeout pero sin configuracion de transport. Go usa un `DefaultTransport` con `MaxIdleConnsPerHost: 2`.

**Propuesta:** Configurar transport explicito:

```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
}
```

**Impacto:** Reutilizacion de conexiones TCP/TLS, eliminando overhead de handshake en llamadas consecutivas.

### 5.2 Reduccion de fallas

#### 5.2.1 Circuit breaker para adapters externos

**Estado actual:** Si Ollama cae, cada request intenta conectar y falla con timeout de 30s. El worker se bloquea 30s por mensaje antes de hacer NAK.

**Propuesta:** Implementar circuit breaker por adapter:

```
Cerrado → N fallas consecutivas → Abierto (fail fast, 0ms) → cooldown → Semi-abierto → probe
```

**Impacto:** En lugar de esperar 30s * 10 reintentos = 5 minutos por mensaje cuando Ollama cae, el circuit breaker falla en <1ms despues de detectar la caida.

#### 5.2.2 Dead letter queue para errores permanentes

**Estado actual:** Los errores permanentes (`ErrPermanent`) se ACK y el mensaje se pierde. No hay visibilidad de que registros fallaron indexacion.

**Propuesta:** Publicar a un subject `record.failed` con el error y el record_id. Crear un consumer de monitored/alerta sobre ese subject.

#### 5.2.3 Retry inteligente con jitter

**Estado actual:** NATS reintenta con backoff fijo (AckWait: 60s). 10 consumers compitiendo por reintentar al mismo segundo generan thundering herd.

**Propuesta:** Implementar exponential backoff con jitter en el handler del worker:

```
Intento 1: 1s ± jitter
Intento 2: 2s ± jitter
Intento 3: 4s ± jitter
...
Intento 10: 512s (cap a 300s)
```

#### 5.2.4 Validacion de embeddings

**Estado actual:** El pipeline confia en que Ollama devuelve vectores validos (768 dimensiones, valores finitos). No hay validacion.

**Propuesta:** Validar en el motor (no en el adapter) que:
- La dimension es la esperada
- No hay valores NaN o Inf
- La norma L2 esta en rango razonable (deteccion de vectores degenerados)

### 5.3 Reduccion de tokens (LLM)

#### 5.3.1 Contexto comprimido para el LLM

**Estado actual:** El `QueryService` ensambla el contexto concatenando chunks recuperados como texto plano. Con 10 chunks de 500 caracteres = ~5000 caracteres = ~1500 tokens de contexto.

**Propuestas de reduccion:**

| Tecnica | Reduccion estimada | Implementacion |
|---------|-------------------|----------------|
| Deduplicacion de chunks similares | 10-30% | Threshold de similitud coseno entre chunks recuperados |
| Truncamiento inteligente por relevancia | 20-40% | Solo incluir chunks con score > threshold dinamico |
| Resumen previo de chunks (pre-LLM) | 40-60% | Un LLM local (rapido, barato) resume antes de enviar al LLM principal |
| Template de prompt optimizado | 5-15% | Reducir instrucciones del system prompt, usar JSON schema |

#### 5.3.2 Cache de consultas frecuentes

**Estado actual:** Redis esta declarado como dependencia pero no implementado.

**Propuesta:** Cache de dos niveles:

```
Nivel 1: Cache de embeddings de consulta (evita re-embedding de la misma pregunta)
Nivel 2: Cache de respuesta completa (key: hash de pregunta + filtros, TTL: configurable)
```

**Invalidacion:** Cuando se indexa un nuevo registro que matchea el entity_ref de una consulta cacheada, invalidar esa entrada.

**Impacto:** Consultas repetidas (comunes en operaciones: "¿que paso con el lote X?" se pregunta multiples veces) pasan de ~2-5s a <10ms.

#### 5.3.3 Seleccion dinamica de modelo

**Estado actual:** Un solo modelo LLM para todas las consultas. Claude Sonnet para "¿cuantos registros hay?" es overkill.

**Propuesta:** Clasificar la consulta ANTES de enviar al LLM:

```
Consulta simple (conteo, ultima fecha, existencia) → modelo local pequeno o query SQL directo
Consulta compleja (resumen, analisis, correlacion) → modelo externo (Claude/GPT)
```

**Impacto:** 60-70% de las consultas operativas son simples. Atenderlas sin LLM externo elimina latencia, costo y tokens.

#### 5.3.4 Prompt engineering: formato estructurado

**Estado actual:** El system prompt pide respuesta JSON pero no usa JSON schema ni few-shot examples optimizados.

**Propuesta:**
- Usar JSON schema en el system prompt (reduce ambiguedad → menos tokens de respuesta)
- Eliminar instrucciones redundantes del prompt
- Usar separadores estructurados en el contexto en vez de texto narrativo:

```
Actual:
  "El registro R-001 del 2024-03-15 indica que el lote L-001 fue inspeccionado..."

Propuesto:
  [R-001|2024-03-15|lot:L-001|inspection] Lote inspeccionado, sin defectos.
```

**Impacto estimado:** 30-50% de reduccion en tokens de contexto sin perdida de informacion semantica.

---

## 6. Capa de Ingesta Implementada: CLI

El cuello de botella del motor no era la indexacion ni la consulta --- era la puerta de entrada. La unica forma de ingresar datos era via `POST /api/v1/records`. Se implemento un CLI (`cmd/toi/`) como primera capa de conveniencia:

```
$ toi ingest file invoices.csv --source erp --type invoice
✓ 1,847 records ingested in 2.3s

$ toi ingest bulk records.jsonl
✓ 12,340 records ingested in 4.2s

$ toi query "What shipments were delayed last week?"
Found 12 records across 3 sources...

$ toi watch /data/scans --source scanner --type scan
Watching /data/scans for new files...
```

### 6.1 Arquitectura del CLI

El CLI es un adapter HTTP puro --- no introduce logica de dominio. Llama a la API existente:

```
cmd/toi/
  main.go      ← Entry point, subcommand routing
  client.go    ← HTTP client para la API del motor
  ingest.go    ← Importadores CSV, JSON, JSONL
  query.go     ← Comandos query y search
  watch.go     ← Directory watcher con polling
```

### 6.2 Mapeo CSV → Record

El importador CSV reconoce columnas conocidas (`source`, `record_type`, `occurred_at`, `entity_ref`, `actor_ref`, `title`, `tags`) y mapea todo lo demas a `Payload`. Esto permite importar datos historicos desde cualquier hoja de calculo sin transformacion previa.

### 6.3 Evaluacion de OpenTelemetry

Se evaluo OTel como mecanismo de ingesta. Conclusion: **no aplica**. OTel instrumenta codigo en ejecucion (traces, metrics, logs). Las fuentes del motor son archivos, ERPs, scanners, emails y humanos --- no apps instrumentables. Se adoptaron dos conceptos: **Semantic Conventions** (vocabulario estandar para atributos) y el **Collector pattern** (receiver → processor → exporter).

### 6.4 Modelos de referencia

| Modelo | Patron | Aplicabilidad al motor |
|--------|--------|----------------------|
| **Datadog** | Agents que corren donde estan los datos, push de metricas | Nivel 3 (v2): agents para edge |
| **Algolia** | Conectores pre-armados para fuentes comunes (Shopify, Salesforce) | Nivel 2: conectores especificos |
| **OTel Collector** | Hibrido receiver/processor/exporter | Patron de adapter adoptable |
| **HOL.org** | Registro universal + trust scoring sobre blockchain | Validacion del patron "ingestá, normalizá, descubrí" |

---

## 7. Adapters propuestos para conectar la realidad

### 7.1 WhatsApp Business API Adapter

**Port que implementaria:** Nuevo port `CaptureAdapter` o reutilizacion del endpoint HTTP como intermediario.

```
WhatsApp webhook → Message Parser → Record mapping → RecordService.Ingest()
                                   → Media download → MinIO storage
                                   → Audio → Whisper → transcription
```

**Mapping de mensajes a Records:**

| Tipo WhatsApp | RecordType | Procesamiento adicional |
|---------------|-----------|------------------------|
| Texto | `note` | Extraccion de entidades del texto |
| Foto | `photo` | OCR (Tesseract), almacenamiento (MinIO) |
| Audio | `audio` | Transcripcion (Whisper), luego como texto |
| Documento | `document` | Parsing segun tipo (PDF, Excel), almacenamiento |
| Ubicacion | `location` | Coordenadas en payload |
| Video | `video` | Almacenamiento, thumbnail |

**Source:** `whatsapp:{phone_number}`  
**ActorRef:** `whatsapp:{sender_phone}`  
**EntityRef:** Extraido del nombre del grupo o hashtags en el mensaje.

### 7.2 Whisper Adapter (transcripcion)

**Port que implementaria:** Nuevo port `Transcriber`:

```go
type Transcriber interface {
    Transcribe(ctx context.Context, audio []byte, format string) (string, error)
}
```

Implementable con Whisper local (via Ollama o whisper.cpp) o externo (OpenAI Whisper API).

### 7.3 OCR Adapter

**Port que implementaria:** Nuevo port `TextExtractor`:

```go
type TextExtractor interface {
    Extract(ctx context.Context, image []byte, format string) (string, error)
}
```

Implementable con Tesseract (local) o Google Cloud Vision (externo).

### 7.4 Email Adapter

**Mecanismo:** IMAP polling o recepcion SMTP directa.

```
Email entrante → Parser (subject, body, attachments) → Record(s)
```

Un email puede generar multiples Records (uno por adjunto + uno por el cuerpo).

---

## 8. Matriz de independencia Motor/Adapter

Esta tabla demuestra que el motor y los adapters evolucionan de forma independiente:

| Cambio en el Motor | Afecta Adapters? | Por que |
|-------------------|-----------------|---------|
| Agregar campo a Record | NO | Los adapters leen la interfaz, no la struct directamente |
| Cambiar estrategia de chunking | NO | `Chunker` es un port interno del motor |
| Optimizar extraccion de entidades | NO | Logica interna del `QueryService` |
| Cambiar formato de checksum (SHA-256 → SHA-512) | NO | El checksum se calcula en el dominio |
| Agregar nuevo tipo de filtro a `SearchFilter` | SI (minor) | `IndexStore.SearchSimilar` recibe el filtro |

| Cambio en Adapter | Afecta Motor? | Por que |
|-------------------|--------------|---------|
| Migrar de PostgreSQL a CockroachDB | NO | Mismo port `RecordStore` |
| Migrar de NATS a Kafka | NO | Mismo port `EventPublisher` |
| Cambiar de Ollama a OpenAI embeddings | NO | Mismo port `Embedder` |
| Agregar adapter de WhatsApp | NO | Usa `RecordService.Ingest()` existente |
| Cambiar de chi a stdlib net/http | NO | Solo afecta `platform/http` |

---

## 9. Metricas de referencia actuales

| Metrica | Valor actual | Target optimizado |
|---------|-------------|-------------------|
| Ingesta (POST /records) | ~5-10ms | ~3-5ms (pool tuning) |
| Busqueda semantica (/search) | 100-200ms | 50-100ms (HNSW tuning + cache) |
| Consulta LLM (/query) | 2-5s | 500ms-2s (cache + modelo adaptativo) |
| Indexacion (worker) | ~1-2s/record | ~200ms/record (batching) |
| Auth (API key validation) | ~100ms * n_keys | ~100ms constante (prefix cache) |
| Tokens por consulta (contexto) | ~1500-2000 | ~500-800 (compresion + dedup) |

---

## 10. Conclusiones

1. **El Motor es el activo intelectual.** El modelo de registro universal, el pipeline de indexacion y el motor de consulta son logica de dominio pura, portable y testeable sin infraestructura.

2. **Los Adapters son intercambiables por diseno.** La arquitectura hexagonal garantiza que se puede reemplazar cualquier componente de infraestructura sin alterar el motor. PostgreSQL puede ser CockroachDB. NATS puede ser Kafka. Ollama puede ser OpenAI.

3. **La realidad es agnostica al motor.** Un mensaje de WhatsApp, un email, un escaneo y un sensor IoT entran al mismo motor por el mismo port (`RecordStore.Append`). El motor no sabe ni le importa de donde vienen los datos.

4. **Las optimizaciones del motor no alteran los adapters.** Mejorar chunking, ajustar HNSW, comprimir contexto de tokens — todo ocurre dentro del boundary del dominio. Los adapters siguen funcionando sin cambios.

5. **El proximo paso critico es el adapter de captura.** El motor esta listo. Lo que falta es el puente entre la realidad operativa (WhatsApp, email, sensores) y el motor. Ese puente es un adapter, no una modificacion del motor.

---

## Apendice A: Arbol de dependencias del Motor

```
record/
  ├── record.go          (modelo, 0 imports externos)
  ├── service.go         (RecordStore port, EventPublisher port)
  └── validation.go      (reglas de negocio, 0 imports externos)

indexing/
  ├── chunk.go           (modelo, 0 imports externos)
  ├── chunker.go         (estrategias, 0 imports externos)
  ├── pipeline.go        (RecordStore port, Chunker port, Embedder port, IndexStore port)
  └── worker.go          (Pipeline, 0 imports de adapter)

query/
  ├── retriever.go       (Embedder port, IndexStore port)
  ├── service.go         (Retriever, LanguageModel port)
  ├── entity.go          (extraccion regex, 0 imports externos)
  └── context.go         (ensamblado de contexto, 0 imports externos)

auth/
  ├── apikey.go          (modelo, 0 imports externos)
  └── store.go           (APIKeyStore port, 0 imports externos)
```

**Total de imports externos en el motor: 2** — `github.com/google/uuid` y la stdlib de Go. Ningun framework, ningun driver, ninguna dependencia de infraestructura.

## Apendice B: Roadmap de optimizacion priorizado

| Prioridad | Optimizacion | Esfuerzo | Impacto |
|-----------|-------------|----------|---------|
| 1 | API key prefix cache | 2h | Alto: auth O(1) vs O(n) |
| 2 | Batch processing en worker | 4h | Alto: throughput 5-10x |
| 3 | Circuit breaker para Ollama/LLM | 3h | Alto: resiliencia |
| 4 | Cache de embedding de consulta (Redis) | 4h | Medio: elimina re-embedding |
| 5 | Prompt compression (formato compacto) | 2h | Medio: 30-50% menos tokens |
| 6 | Clasificacion pre-LLM de consultas | 6h | Alto: 60-70% consultas sin LLM externo |
| 7 | Dead letter queue | 2h | Medio: visibilidad de fallas |
| 8 | HNSW tuning dinamico | 1h | Bajo-medio: depende de volumen |
| 9 | Cache de respuesta completa (Redis) | 4h | Medio: consultas repetidas <10ms |
| 10 | Validacion de embeddings | 1h | Bajo: prevencion de data corrupta |
