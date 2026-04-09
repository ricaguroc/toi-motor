# Trazabilidad Operativa Inteligente: Un Motor de Registro Universal con Consulta en Lenguaje Natural

**Versión:** 1.0  
**Fecha:** Abril 2026  
**Autor:** Ricardo Agurto  
**Licencia:** Apache 2.0

---

## 1. Resumen

Este documento describe la arquitectura, modelo de datos y mecanismos internos de Trazabilidad Operativa Inteligente (TOI): un motor backend que digitaliza la realidad operativa de una organización y la vuelve consultable en lenguaje natural. El sistema opera en tres capas: (1) un almacen de registros universales, inmutable y append-only, que captura cualquier hecho operativo sin imponer esquema; (2) un pipeline de indexacion basado en Retrieval-Augmented Generation (RAG) que transforma registros en vectores semanticos buscables; y (3) una interfaz de consulta que traduce preguntas en lenguaje natural a respuestas fundamentadas, citadas y con evaluacion de confianza. El motor esta implementado en Go 1.22 con arquitectura hexagonal, PostgreSQL 16 con pgvector como almacen unificado (relacional + vectorial + full-text search), NATS JetStream para procesamiento asincrono, Ollama para embeddings locales, y un modelo de lenguaje (LLM) externo y agnostico al proveedor. El resultado es infraestructura: no un sistema de negocio, sino la capa sobre la cual cualquier sistema de trazabilidad puede construirse.

---

## 2. Introduccion

### 2.1 El problema: la realidad operativa fragmentada

En la mayoria de las organizaciones, la informacion operativa esta dispersa entre cuadernos, hojas de calculo, mensajes de WhatsApp, correos electronicos, sistemas legacy y la memoria de las personas. Cuando alguien necesita saber que paso con un lote, un equipo, un pedido o un incidente, la respuesta requiere reconstruccion manual: buscar en multiples fuentes, preguntar a colegas, cruzar datos.

Este problema no es tecnologico en su origen --- es estructural. Las organizaciones generan informacion operativa de forma continua, pero carecen de un punto unico donde esa informacion se registre de manera uniforme e inmutable.

### 2.2 Por que ahora

Tres factores convergen para hacer viable esta solucion:

1. **Madurez de modelos de lenguaje.** Los LLMs (Claude, GPT, Llama, Mistral, etc.) alcanzan calidad suficiente para interpretar contexto operativo y responder preguntas fundamentadas. El ecosistema permite elegir entre ejecucion local o APIs externas segun las necesidades de cada organizacion.

2. **Bases de datos vectoriales embebidas.** PostgreSQL con pgvector permite almacenar embeddings y ejecutar busqueda por similitud coseno dentro de la misma base que contiene los datos relacionales, eliminando la complejidad operativa de mantener un sistema vectorial separado.

3. **Avance regulatorio.** Las regulaciones de trazabilidad avanzan hacia lo digital. La capacidad de registrar, buscar y auditar la realidad operativa dejara de ser una ventaja competitiva para convertirse en un requisito.

---

## 3. Modelo Universal de Registro

### 3.1 El Record como atomo del sistema

El nucleo del motor es un modelo de datos unico y universal: el `Record`. Cualquier hecho operativo --- un escaneo de codigo de barras, un movimiento de inventario, un documento adjunto, una nota de campo, una foto, un ticket --- se representa como un registro con la misma estructura:

```go
type Record struct {
    ID         uuid.UUID      // clave primaria interna
    RecordID   uuid.UUID      // identificador publico estable
    OccurredAt time.Time      // cuando ocurrio el hecho
    IngestedAt time.Time      // cuando el sistema lo registro
    Source     string         // origen: "scanner_app", "erp_webhook", "manual"
    RecordType string         // tipo: "scan", "movement", "document", "note", "photo"
    EntityRef  *string        // referencia a entidad: "lot:L-2024-001", "equipment:PUMP-01"
    ActorRef   *string        // referencia al actor: "user:maria@empresa.com"
    Title      *string        // titulo descriptivo
    Payload    map[string]any // contenido libre, esquema abierto (JSONB)
    ObjectRefs []string       // referencias a archivos en almacenamiento de objetos
    Checksum   string         // SHA-256 para deteccion de alteracion
    Tags       []string       // etiquetas libres
    Metadata   map[string]any // metadatos adicionales
}
```

Esta estructura es deliberadamente generica. El campo `Payload` acepta cualquier JSON sin esquema predefinido, lo que permite que fuentes heterogeneas --- desde un escaner portatil hasta un webhook de ERP --- alimenten el mismo almacen sin transformaciones previas.

### 3.2 Separacion de identidad: ID vs RecordID

El modelo distingue entre `ID` (clave primaria interna, nunca expuesta) y `RecordID` (identificador publico estable). Esta separacion permite que las APIs externas referencien registros de forma estable mientras la capa de persistencia mantiene control total sobre su esquema interno.

### 3.3 Inmutabilidad y append-only

El almacen de registros es estrictamente append-only. La interfaz del puerto `RecordStore` lo hace explicito a nivel de compilacion:

```go
type RecordStore interface {
    Append(ctx context.Context, r Record) error
    GetByID(ctx context.Context, recordID uuid.UUID) (Record, error)
    List(ctx context.Context, filter Filter) (ListResult, error)
}
```

No existen metodos `Update`, `Delete` ni `Patch`. La inmutabilidad no es una convencion --- esta reforzada en el tipo del puerto. Un registro, una vez persistido, no puede ser modificado ni eliminado.

Esta decision tiene consecuencias importantes:

- **Auditabilidad completa.** El almacen funciona como un ledger: cada hecho queda registrado tal como fue recibido.
- **Simplificacion de concurrencia.** Sin actualizaciones ni borrados, los conflictos de escritura desaparecen.
- **Compatibilidad con regulaciones.** Los requisitos de trazabilidad demandan frecuentemente registros que no puedan ser alterados retroactivamente.

### 3.4 El formato entity_ref / actor_ref

Las referencias a entidades y actores siguen un formato `tipo:identificador`:

| Formato | Ejemplo | Significado |
|---------|---------|-------------|
| `lot:L-2024-001` | Lote L-2024-001 | Lote de produccion |
| `equipment:PUMP-01` | Bomba 01 | Equipo industrial |
| `vehicle:TRK-103` | Camion 103 | Vehiculo de transporte |
| `order:ORD-50012` | Orden 50012 | Orden de compra/trabajo |
| `user:maria@empresa.com` | Maria | Actor humano |
| `device:scanner-003` | Scanner 3 | Dispositivo automatizado |

Este formato permite al motor de consulta extraer entidades de preguntas en lenguaje natural y filtrar registros por entidad de forma eficiente, sin depender de tablas de maestros ni catalogos predefinidos.

### 3.5 Checksum para deteccion de alteracion

Cada registro recibe un checksum SHA-256 calculado de forma deterministica sobre sus campos canonicos:

```go
func ComputeChecksum(r Record) (string, error) {
    payloadJSON, err := marshalSorted(r.Payload)
    // pre-image: record_id | occurred_at (RFC3339Nano, UTC) | source | record_type | payload (sorted JSON)
    pre := strings.Join([]string{
        r.RecordID.String(),
        r.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
        r.Source,
        r.RecordType,
        string(payloadJSON),
    }, "|")
    sum := sha256.Sum256([]byte(pre))
    return hex.EncodeToString(sum[:]), nil
}
```

El payload se serializa con claves ordenadas alfabeticamente en todos los niveles de anidacion, garantizando que el mismo contenido siempre produzca el mismo hash independientemente del orden de iteracion de los mapas de Go. La validacion de integridad a nivel de base de datos exige exactamente 64 caracteres hexadecimales (`CHECK (char_length(checksum) = 64)`).

### 3.6 Event sourcing vs. registro de hechos

El modelo se asemeja a event sourcing pero tiene una diferencia fundamental: no reconstruye estado. Event sourcing captura eventos para derivar el estado actual de una entidad mediante replay. TOI registra hechos operativos para hacerlos consultables. No hay agregados, no hay proyecciones, no hay replay. El registro ES el dato final --- no un medio para llegar a otro estado.

---

## 4. Pipeline de Indexacion (RAG)

### 4.1 Flujo general

Cuando un registro es persistido, el `RecordService` publica un evento `record.ingested` a NATS JetStream. Un worker independiente consume estos eventos y ejecuta el pipeline de indexacion:

```
record.ingested (NATS) → fetch record → generar texto → chunking → embedding → upsert pgvector
```

El pipeline esta implementado en la estructura `Pipeline` del paquete `indexing`:

```go
type Pipeline struct {
    store    record.RecordStore  // para obtener el registro completo
    chunker  Chunker             // estrategia de fragmentacion
    embedder Embedder            // modelo de embedding
    index    IndexStore          // almacen vectorial
    maxRetry int                 // intentos maximos (default: 5)
}
```

### 4.2 Representacion textual con cabecera de metadatos

Antes de fragmentar un registro, se genera una representacion textual estructurada que incluye una cabecera con metadatos criticos:

```
[RECORD TYPE: scan] [SOURCE: scanner_app] [OCCURRED: 2024-03-15T14:30:00Z]
ENTITY: lot:L-2024-001
ACTOR: user:maria@empresa.com

TITLE: Escaneo de ingreso al almacen

DETAILS:
- location: warehouse-A
- status: accepted
- weight_kg: 1250.5

TAGS: ingreso, almacen, calidad
```

Esta representacion tiene dos propositos: (1) proporcionar al modelo de embedding contexto semantico rico mas alla del contenido textual puro, y (2) garantizar que cada chunk, incluso los fragmentos de documentos largos, contenga la cabecera de metadatos necesaria para que el modelo de lenguaje pueda citarlos correctamente.

El payload se aplana recursivamente usando notacion de punto para claves anidadas (`parent.child.field`), y las listas se unen con comas. Las claves se ordenan alfabeticamente para garantizar representaciones deterministas.

### 4.3 Estrategias de chunking

No todos los registros se fragmentan igual. La estrategia depende del tipo de registro:

| RecordType | Estrategia | Configuracion |
|------------|-----------|---------------|
| `scan`, `movement`, `photo` | Sin fragmentacion | Chunk unico |
| `note` | Parrafo | Division en `\n\n`, fusion si < 512 tokens |
| `ticket`, `log` | Ventana deslizante | 512 tokens, 50 tokens de overlap |
| `document`, `email` | Ventana deslizante | 512 tokens, 100 tokens de overlap |

La ventana deslizante con overlap asegura que la informacion en los bordes de fragmento no se pierda. Para documentos y emails --- que tienden a ser mas largos y con estructura narrativa --- se usa un overlap mayor (100 tokens) para preservar coherencia contextual.

Cada chunk de continuacion incluye la cabecera de metadatos del registro original prepended, de modo que ningun fragmento quede huerfano de contexto. La estimacion de tokens usa la heuristica de 1 token por cada 4 caracteres.

### 4.4 Modelo de embedding: nomic-embed-text (local)

El motor usa `nomic-embed-text` ejecutado localmente via Ollama para generar embeddings. Es importante distinguir: Ollama se usa UNICAMENTE para embeddings (la conversion de texto a vectores numericos), NO para el modelo de lenguaje natural. Son dos funciones distintas de IA:

- **Embedding** = calculo matematico que convierte texto en un vector de 768 numeros. No "entiende" ni genera texto. Permite buscar por similitud semantica. Se ejecuta localmente porque es liviano (~270MB) y se invoca miles de veces por dia.
- **LLM** = modelo de lenguaje que interpreta preguntas y genera respuestas. Se ejecuta externamente (ver Seccion 5) porque es pesado y se invoca bajo demanda.

nomic-embed-text fue elegido por:

- **Rendimiento en textos operativos cortos.** Los registros tipicos (escaneos, movimientos, notas breves) rara vez exceden 512 tokens. nomic-embed-text esta optimizado para este rango.
- **Ejecucion local.** No requiere APIs externas. Los datos operativos nunca salen de la infraestructura del cliente.
- **Eficiencia de recursos.** Modelo ligero (~270MB), ejecutable en hardware modesto sin GPU dedicada.

Los embeddings se procesan en batches de 32 para optimizar throughput y reducir overhead de llamadas al modelo.

### 4.5 Almacenamiento vectorial: pgvector con HNSW

Los embeddings se almacenan en PostgreSQL usando la extension pgvector. El indice utiliza HNSW (Hierarchical Navigable Small World) con los parametros:

```sql
CREATE INDEX record_embeddings_hnsw_idx
    ON record_embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);
```

- **m = 16**: numero de conexiones bidireccionales por nodo. Valor estandar que balancea precision y uso de memoria.
- **ef_construction = 64**: factor de expansion durante la construccion del indice. Valores mas altos producen indices mas precisos a costa de tiempo de construccion.
- **vector_cosine_ops**: operador de distancia coseno, alineado con la metrica para la que nomic-embed-text fue entrenado.

La tabla `record_embeddings` desnormaliza campos del registro original (`entity_ref`, `actor_ref`, `record_type`, `occurred_at`) para permitir filtrado pre-vectorial sin necesidad de JOIN:

```sql
CREATE TABLE record_embeddings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    record_id    UUID NOT NULL REFERENCES records(record_id) ON DELETE RESTRICT,
    chunk_index  INT  NOT NULL DEFAULT 0,
    chunk_text   TEXT NOT NULL,
    embedding    vector(768) NOT NULL,
    entity_ref   TEXT,
    actor_ref    TEXT,
    record_type  TEXT NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT record_embeddings_unique_chunk UNIQUE (record_id, chunk_index)
);
```

La restriccion `ON DELETE RESTRICT` refuerza la inmutabilidad: no se puede eliminar un registro que tenga embeddings asociados.

### 4.6 Pipeline asincrono via NATS JetStream

La publicacion del evento `record.ingested` y su consumo por el pipeline de indexacion ocurren de forma asincrona. Esto tiene implicaciones importantes:

- **No bloquea la ingesta.** El endpoint `POST /api/v1/records` retorna inmediatamente despues de persistir el registro. La indexacion ocurre en background.
- **Resiliencia.** Si el pipeline falla (modelo no disponible, error transitorio de base de datos), NATS JetStream retiene el mensaje para reentrega.
- **Escalabilidad independiente.** Los workers de indexacion pueden escalar horizontalmente sin afectar la API de ingesta.

### 4.7 Reintento y manejo de errores

El pipeline distingue entre errores permanentes y transitorios:

- **Permanentes** (`ErrPermanent`): JSON malformado, registro no encontrado, fallo de chunking. El mensaje se descarta --- reintentar no resolveria el problema.
- **Transitorios** (`ErrTransient`): modelo de embedding no disponible, error de red, fallo de escritura a base de datos. Se reintenta hasta 5 veces con backoff exponencial (1s, 2s, 4s, 8s, 16s).

```go
func (p *Pipeline) withRetry(ctx context.Context, fn func() error) error {
    var lastErr error
    for attempt := 0; attempt < p.maxRetry; attempt++ {
        if attempt > 0 {
            delay := time.Duration(1<<(attempt-1)) * time.Second
            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return fmt.Errorf("%w: context cancelled", ErrTransient)
            }
        }
        lastErr = fn()
        if lastErr == nil {
            return nil
        }
    }
    return fmt.Errorf("%w: exhausted %d retries", ErrTransient, p.maxRetry)
}
```

La cancelacion de contexto aborta el ciclo de reintentos, permitiendo shutdown limpio del worker.

---

## 5. Motor de Consulta

### 5.1 Flujo de consulta

Una consulta en lenguaje natural atraviesa cinco etapas:

```
pregunta del usuario
    → extraccion de entidades
    → embedding de la pregunta
    → busqueda vectorial con filtros
    → ensamblado de contexto
    → completacion LLM
    → respuesta estructurada JSON
```

### 5.2 Extraccion de entidades

Antes de buscar por similitud semantica, el motor analiza la pregunta del usuario para detectar referencias a entidades conocidas. Ocho patrones regex cubren los tipos mas comunes:

| Patron | Ejemplo | Referencia generada |
|--------|---------|---------------------|
| `lot:` / `lote:` prefix | `lote:L-2024-001` | `lot:L-2024-001` |
| `L-YYYY-NNN` | `L-2024-001` | `lot:L-2024-001` |
| `LP-NNNN` | `LP-0042` | `lot:LP-0042` |
| `PUMP-NN` / `EQ-NNN` | `PUMP-01` | `equipment:PUMP-01` |
| `TRK-NNN` | `TRK-103` | `vehicle:TRK-103` |
| `ORD-NNNNN` / `OC-NNN` | `ORD-50012` | `order:ORD-50012` |
| `P-NNN` | `P-091` | `equipment:P-091` |
| Email | `maria@empresa.com` | `user:maria@empresa.com` |

Cuando se detecta una entidad, se usa como filtro en la busqueda vectorial. Esto reduce drasticamente el espacio de busqueda y mejora la precision: "que paso con el lote L-2024-001" primero filtra por `entity_ref = 'lot:L-2024-001'` y luego busca por similitud dentro de ese subconjunto.

### 5.3 Busqueda vectorial con filtros

El `DefaultRetriever` ejecuta una busqueda de similitud coseno en pgvector, aplicando filtros opcionales de entidad, rango temporal y tipo de registro:

```sql
SELECT record_id, chunk_index, chunk_text,
       1 - (embedding <=> $1) AS score,
       occurred_at, record_type, entity_ref
FROM record_embeddings
WHERE ($2::text IS NULL OR entity_ref = $2)
  AND ($3::text IS NULL OR actor_ref = $3)
  AND ($4::text IS NULL OR record_type = $4)
  AND ($5::timestamptz IS NULL OR occurred_at >= $5)
  AND ($6::timestamptz IS NULL OR occurred_at <= $6)
ORDER BY embedding <=> $1
LIMIT $7
```

El operador `<=>` calcula la distancia coseno; `1 - distancia` produce el score de similitud. Los filtros NULL-safe permiten combinar busqueda semantica con restricciones estructuradas en una sola consulta.

### 5.4 Ensamblado de contexto y deduplicacion

Los chunks recuperados se procesan antes de enviarse al LLM:

1. **Deduplicacion por RecordID.** Si multiples chunks del mismo registro son recuperados, se conserva unicamente el de mayor score. Esto evita que un documento largo con muchos chunks domine el contexto.

2. **Ordenamiento por score.** Los chunks se ordenan por score descendente, priorizando los mas relevantes.

3. **Formateo estructurado.** Cada chunk se envuelve con un encabezado que incluye `record_id`, tipo y fecha:

```
--- RECORD 1 (record_id: 550e8400-..., type: scan, occurred: 2024-03-15T14:30:00Z) ---
[contenido del chunk]
---
```

4. **Truncamiento a 8000 tokens.** El contexto se trunca a ~32,000 caracteres (aproximadamente 8,000 tokens) eliminando los chunks de menor score. Este limite previene que el contexto exceda la ventana del modelo y mantiene un balance entre cobertura y costo de inferencia.

### 5.5 LLM agnostico al proveedor

El motor es agnostico al proveedor de LLM. La interfaz de dominio define un contrato minimo:

```go
type LanguageModel interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
```

Cualquier proveedor que pueda recibir un system prompt y un mensaje de usuario, y devolver texto, satisface este contrato. La implementacion actual usa la API de Anthropic (Claude), pero el motor no sabe ni le importa que LLM hay del otro lado. La configuracion es:

```env
LLM_BASE_URL=https://api.anthropic.com
LLM_API_KEY=sk-ant-xxx
LLM_MODEL=claude-sonnet-4-20250514
```

Si una organizacion prefiere otro proveedor --- OpenAI, Groq, un modelo local via Ollama --- implementa la interfaz `LanguageModel` con su propio adapter. Eso es responsabilidad del usuario del motor, no del motor.

### 5.6 Endpoint /search: consulta directa al RAG

Ademas del endpoint `/query` (que invoca al LLM), el motor expone `POST /api/v1/search` para consulta directa al RAG sin pasar por el LLM. Este endpoint recibe una query de texto, la convierte en embedding, busca por similitud vectorial, y devuelve los registros rankeados por relevancia con su score de similitud.

`/search` es util cuando:
- El LLM no esta configurado o no esta disponible.
- Se necesita velocidad maxima (~100-200ms vs 1-8s con LLM).
- El usuario prefiere interpretar los registros directamente sin intermediacion del LLM.
- Se quiere construir una interfaz de busqueda propia sobre el motor.

El LLM sigue siendo el componente central del motor --- `/search` es una capacidad adicional para quienes necesiten aplicar el motor de forma diferente.

### 5.7 Prompt engineering: reglas de fundamentacion

El system prompt impone seis reglas estrictas al LLM:

```
REGLAS ESTRICTAS:
1. Responde UNICAMENTE con informacion de los registros proporcionados. No inventes datos.
2. Si los registros no contienen suficiente informacion para responder, dilo explicitamente.
3. Cita los record_id relevantes en el campo records_cited.
4. Responde en el mismo idioma de la pregunta del usuario.
5. Evalua tu confianza: "high" si los registros son claros y completos,
   "medium" si son parciales, "low" si son escasos o ambiguos.
6. Identifica "gaps": que informacion faltaria para dar una respuesta completa.
```

Estas reglas implementan el principio de fundamentacion (grounding): el LLM no puede generar informacion que no este en los registros recuperados. La confianza autoasignada y la deteccion de gaps permiten al usuario evaluar la calidad de la respuesta.

### 5.8 Respuesta estructurada

El LLM genera JSON con modo JSON activado. La respuesta se parsea y valida:

```go
type QueryResponse struct {
    Answer            string   `json:"answer"`             // respuesta en lenguaje natural
    Confidence        string   `json:"confidence"`         // "high", "medium", "low"
    RecordsCited      []string `json:"records_cited"`      // UUIDs de registros citados
    Gaps              *string  `json:"gaps"`               // informacion faltante
    SuggestedFollowup []string `json:"suggested_followup"` // preguntas de seguimiento
    RetrievedCount    int      `json:"retrieved_count"`    // chunks recuperados
    QueryMs           int64    `json:"query_ms"`           // latencia total en ms
}
```

Si ningun chunk es recuperado, el motor retorna directamente una respuesta de baja confianza sin invocar al LLM, evitando alucinaciones cuando no hay datos relevantes.

La validacion post-LLM verifica que el campo `confidence` contenga exactamente uno de los tres valores permitidos (`high`, `medium`, `low`), rechazando respuestas que no cumplan el contrato.

---

## 6. Arquitectura

### 6.1 Arquitectura hexagonal / screaming

El codigo esta organizado siguiendo arquitectura hexagonal (ports and adapters) con la variante "screaming" donde los paquetes gritan su intencion de negocio:

```
internal/
  record/       ← dominio: Record, checksum, validacion, RecordStore (port), RecordService
  indexing/     ← dominio: representacion textual, chunker, Embedder (port), IndexStore (port), Pipeline
  query/        ← dominio: extraccion de entidades, Retriever (port), contexto, prompt, QueryService
  auth/         ← dominio: APIKey, APIKeyStore (port), middleware
  platform/     ← adaptadores de infraestructura
    postgres/   ← implementaciones de RecordStore, IndexStore, APIKeyStore
    nats/       ← publicador y consumidor de eventos
    ollama/     ← implementacion de Embedder (embeddings locales)
    anthropic/  ← implementacion de LanguageModel (Claude API)
    http/       ← handlers, router, middleware HTTP
    minio/      ← almacenamiento de objetos
    redis/      ← cache (futuro)

cmd/
  api/          ← servidor HTTP
  worker/       ← consumidor NATS para indexacion
```

Los puertos son interfaces Go definidas en los paquetes de dominio. Los adaptadores los implementan en `platform/`. La verificacion de conformidad es en tiempo de compilacion:

```go
var _ indexing.IndexStore = (*PostgresEmbeddingRepo)(nil)
```

Esta linea en `embedding_repo.go` garantiza que si `PostgresEmbeddingRepo` deja de satisfacer la interfaz `IndexStore`, el codigo no compila.

### 6.2 Por que Go

- **Concurrencia nativa.** Goroutines para publicacion asincrona de eventos, workers de indexacion, y manejo de requests HTTP concurrentes.
- **Binario estatico.** Un solo binario sin dependencias de runtime. Despliegue trivial en contenedores minimos.
- **Tipado fuerte sin overhead.** Las interfaces como contratos de puerto son verificadas en compilacion, no en runtime.
- **Ecosistema maduro para servicios backend.** pgx, chi, slog, nats.go --- bibliotecas probadas y estables.

### 6.3 PostgreSQL como almacen 3-en-1

Una decision arquitectonica central es usar PostgreSQL para tres funciones que frecuentemente se delegan a sistemas separados:

| Funcion | Alternativa tipica | En TOI |
|---------|-------------------|--------|
| Almacen relacional | PostgreSQL | `records` table |
| Busqueda vectorial | Pinecone, Weaviate, Milvus | pgvector + HNSW index |
| Full-text search | Elasticsearch | `tsvector` + GIN index |

Los beneficios de esta consolidacion:

- **Consistencia transaccional.** Registros y embeddings en la misma transaccion. No hay estados intermedios inconsistentes.
- **Complejidad operativa reducida.** Un solo sistema a monitorear, respaldar y escalar.
- **Queries combinadas.** Filtros relacionales (entidad, fecha) y busqueda vectorial en la misma consulta SQL.

El tradeoff es claro: a escala de millones de registros con vectores de alta dimensionalidad, un sistema vectorial dedicado puede superar a pgvector en throughput de busqueda. Pero para el caso de uso objetivo --- trazabilidad operativa de una organizacion individual --- PostgreSQL ofrece rendimiento mas que suficiente con una fraccion de la complejidad operativa.

### 6.4 Arquitectura single-tenant

El motor esta disenado como single-tenant: una instancia por organizacion. Esta decision simplifica:

- **Aislamiento de datos.** No hay riesgo de fuga de datos entre tenants.
- **Rendimiento predecible.** Sin noisy neighbors.
- **Escalamiento simple.** Cada organizacion escala su infraestructura de forma independiente.

El costo es que multiples organizaciones requieren multiples despliegues. Para el caso de uso --- software on-premise o en nube privada para datos operativos sensibles --- este modelo es preferible a multi-tenancy.

### 6.5 Flujo de datos completo

```
Cualquier fuente (app, escaner, sensor, ERP, formulario, email)
        |
        v
+---------------------------+
|   Universal Record Store   |  <- Todo entra como registro
|                           |     append-only, inmutable
|   record_id               |
|   timestamp               |
|   source                  |
|   record_type             |
|   entity_ref              |
|   actor_ref               |
|   payload (libre)         |
+------------+--------------+
             |
             v
+---------------------------+
|       RAG Engine           |  <- Indexa, contextualiza, conecta
|                           |
|   chunking                |
|   embeddings              |
|   busqueda semantica      |
|   retrieval por entidad   |
+------------+--------------+
             |
             v
+---------------------------+
|     LLM (agnostico)        |  <- Interpreta y responde
|                           |
|   "Que paso con X?"       |
|   "Quien movio esto?"     |
|   "Resumi los incidentes  |
|    de esta semana"        |
+---------------------------+
```

---

## 7. Que No Es

Es crucial delimitar lo que el motor NO es, porque su naturaleza generica puede generar confusiones:

- **No es un ERP.** No gestiona ordenes de compra, facturacion ni contabilidad. Registra los hechos que un ERP podria generar.
- **No es un sistema de compliance.** No implementa normas ISO, HACCP ni GMP. Proporciona la capa de datos sobre la cual un sistema de compliance puede construirse.
- **No es un sistema de inventario.** No mantiene conteos de stock ni posiciones de almacen. Registra los movimientos que un sistema de inventario usaria como fuente.
- **No es un sistema de gestion.** No tiene workflows, aprobaciones ni estados. Registra hechos; no los gestiona.

La analogia correcta es infraestructura. PostgreSQL no sabe si almacena datos de un e-commerce o un hospital. Kubernetes no sabe que aplicaciones orquesta. TOI no sabe que tipo de operacion registra. Es la capa neutral de captura y consulta sobre la cual se construyen soluciones de dominio.

---

## 8. Capa de Ingesta (Ingestion Layer)

### 8.1 El problema: la puerta de entrada

El motor esta completo: ingesta via API, indexacion asincrona, consulta en lenguaje natural. Pero la UNICA forma de ingresar datos es via `POST /api/v1/records` con JSON. Esto significa que alguien tiene que escribir codigo para cada fuente de datos. Un ERP no sabe hablar con la API. Un scanner no sabe. Un CSV no se sube solo.

El cuello de botella no es el motor --- es la puerta de entrada.

### 8.2 Evaluacion de OpenTelemetry

Se evaluo OpenTelemetry (OTel) como mecanismo de ingesta. Conclusion: **no aplica a este dominio**.

OTel esta disenado para instrumentar codigo en ejecucion --- una app Go emite traces y logs automaticamente. Pero las fuentes de datos del motor no son apps instrumentables:

| Fuente real | OTel aplica? | Razon |
|-------------|-------------|-------|
| CSV en file server | No | No es una app, es un archivo |
| ERP (SAP, Odoo) | No | No se va a instrumentar SAP |
| Scanner de planta | No | Hardware con protocolo propio |
| Email con adjuntos | No | No es telemetria |
| Notas de operario | No | Texto humano |

Sin embargo, dos conceptos de OTel son adoptables:

1. **Semantic Conventions**: atributos estandar para records (`record.type`, `record.source`, `record.facility`). Vocabulario comun sin cambiar el modelo.
2. **Collector pattern**: el patron receiver → processor → exporter para la capa de adapters.

### 8.3 CLI: la primera capa de ingesta

Se implemento un CLI (`cmd/toi/`) que actua como la primera capa de conveniencia sobre la API HTTP:

```
toi ingest file invoices.csv --source erp --type invoice
toi ingest bulk records.jsonl
toi query "What shipments were delayed last week?"
toi search "lot LP-4821 temperature"
toi watch /data/scans --source scanner --type scan
```

El CLI soporta:

| Comando | Descripcion | Formato |
|---------|-------------|---------|
| `ingest file` | Importa CSV o JSON como records | CSV: columnas conocidas → campos, resto → payload. JSON: objeto o array de IngestRequest |
| `ingest bulk` | Importa JSONL (un record por linea) | Cada linea es un IngestRequest completo |
| `query` | Consulta en lenguaje natural (RAG + LLM) | Texto libre |
| `search` | Busqueda semantica sin LLM | Texto libre |
| `watch` | Monitorea directorio y auto-ingesta archivos nuevos | CSV, JSON, JSONL |

El CLI no introduce logica de dominio nueva --- es un adapter HTTP puro que llama a la API existente. La configuracion se resuelve via flags (`--api-url`, `--api-key`, `--source`, `--type`) o variables de entorno (`TOI_API_URL`, `TOI_API_KEY`).

### 8.4 Mapeo CSV a Record

El importador CSV reconoce columnas que mapean a campos del Record:

| Columna CSV | Campo Record | Comportamiento |
|-------------|-------------|----------------|
| `source` | Source | Usa valor de la columna, o `--source` como default |
| `record_type` | RecordType | Usa valor de la columna, o `--type` como default |
| `occurred_at` | OccurredAt | Parsea RFC3339, ISO date, o DD/MM/YYYY |
| `entity_ref` | EntityRef | Valor directo |
| `actor_ref` | ActorRef | Valor directo |
| `title` | Title | Valor directo |
| `tags` | Tags | Separadas por coma |
| Cualquier otra | Payload[nombre_columna] | Valor como string en payload |

Esto permite importar datos historicos desde cualquier hoja de calculo exportada a CSV sin transformacion previa.

### 8.5 Roadmap de ingesta

La estrategia de ingesta sigue tres niveles:

| Nivel | Que | Cuando |
|-------|-----|--------|
| **1. CLI** | `toi ingest file/bulk`, `toi watch` | Implementado |
| **2. Conectores** | Webhook receiver, email ingester, API adapters | Proximo paso |
| **3. Agents** | Procesos autonomos que corren donde estan los datos y empujan records al motor (modelo Datadog) | V2, con clientes reales |

### 8.6 Comparacion con modelos de referencia

| Modelo | Patron | Aplicabilidad |
|--------|--------|---------------|
| **Datadog Agents** | Push: agente corre donde estan los datos, empuja metricas | Nivel 3: para edge deployment |
| **OTel Collector** | Hibrido: recibe push y hace pull, procesa y exporta | Patron de adapter adoptable |
| **Algolia Connectors** | Pull: conectores pre-armados para fuentes comunes | Nivel 2: conectores especificos |

El motor adopta el mejor de cada modelo segun la fase de madurez del producto.

---

## 9. Trabajo Futuro

### 9.1 OCR para registros fisicos

Integracion de reconocimiento optico de caracteres para digitalizar documentos fisicos --- remitos, certificados de calidad, notas manuscritas --- e ingestarlos como registros con payload extraido automaticamente.

### 9.2 Busqueda hibrida

Combinar busqueda vectorial con full-text search (ya soportado por la columna `search_vector` con tokenizador espanol) usando reciprocal rank fusion (RRF). Esto mejoraria la recuperacion de terminos exactos (codigos, numeros de lote) que la busqueda semantica sola puede perder.

### 9.3 Memoria de sesion

Soporte para contexto conversacional multi-turno, donde el motor recuerda preguntas anteriores de la sesion para resolver referencias anaforicas ("y que mas paso ese dia?").

### 9.4 Importacion masiva (completado parcialmente)

Endpoint para bulk import de registros historicos desde CSV, Excel o exports de sistemas legacy, con pipeline de validacion, deduplicacion y checksum en batch.

### 9.5 Respuestas en streaming

Streaming de la respuesta del LLM via Server-Sent Events (SSE) para mejorar la percepcion de latencia en la interfaz de usuario.

### 9.6 Cadena de hash para evidencia de integridad

Encadenar los checksums de registros consecutivos (cada checksum incluye el checksum del registro anterior), creando una cadena verificable de integridad similar a un blockchain simplificado. Esto elevaria la deteccion de alteracion de individual (por registro) a colectiva (la cadena completa se rompe si un registro intermedio es modificado).

### 9.7 Graph de relaciones entre entidades

Derivar automaticamente un grafo de relaciones entre entidades a partir de los registros (que equipos se usan en que lotes, que personas interactuan con que ordenes), habilitando consultas de tipo "que entidades estan conectadas a X".

---

## 10. Conclusion

Trazabilidad Operativa Inteligente no es un producto de negocio --- es infraestructura. Su tesis fundamental es simple:

> **La realidad, cuando es correctamente registrada, puede ser consultada como si fuera un sistema.**

El motor materializa esta tesis en tres capas que se refuerzan mutuamente: un almacen inmutable que captura la realidad sin imponerle estructura, un pipeline de indexacion que la vuelve buscable semanticamente, y una interfaz de lenguaje natural que la hace accesible a cualquier persona sin conocimiento tecnico.

Las decisiones de diseno --- append-only, esquema abierto, arquitectura hexagonal, PostgreSQL como almacen unificado, embeddings locales, LLM agnostico, single-tenant --- no son accidentales. Cada una refleja una prioridad explicita: la realidad operativa de una organizacion es demasiado critica para depender de cajas negras o sistemas que pueden modificar datos retroactivamente. Los embeddings se ejecutan localmente para que los datos nunca salgan de la infraestructura. El LLM es externo y configurable porque cada organizacion tiene sus propias necesidades y preferencias.

El resultado es una pieza de infraestructura que cualquier equipo puede desplegar, extender y auditar. No requiere depender de un proveedor. No requiere enviar datos a la nube. No requiere confiar en un sistema cerrado para registrar lo que realmente paso en la operacion.

La operacion ya genera los datos. Solo falta registrarlos correctamente.

---

*Publicacion tecnica defensiva. Este documento establece arte previo (prior art) sobre la arquitectura, modelo de datos y mecanismos descritos. Apache 2.0.*
