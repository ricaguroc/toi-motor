# Trazabilidad Operativa Inteligente — Release Tecnico v2

**Version:** 2.0  
**Fecha:** Abril 2026  
**Autor:** Ricardo Agurto  
**Licencia:** Apache 2.0  

---

## 1. Estado del Motor

El motor de Trazabilidad Operativa Inteligente es funcional. Las tres capas del sistema --- ingesta, indexacion semantica y consulta --- operan end-to-end con datos reales. Se han ejecutado pruebas de integracion que verifican el flujo completo: desde la insercion de un registro operativo via HTTP, pasando por su indexacion asincrona mediante NATS y Ollama, hasta la consulta semantica por similitud coseno en pgvector. Los resultados confirman que el motor entiende semanticamente el contenido de los registros y los rankea correctamente por relevancia. El sistema esta respaldado por 119 funciones de test unitario que cubren los cuatro dominios y los adaptadores criticos.

---

## 2. Arquitectura Implementada

El motor esta construido en **Go 1.25** siguiendo una arquitectura **hexagonal/screaming**: los paquetes de dominio definen puertos (interfaces) y los adaptadores de plataforma los implementan. No hay dependencia directa entre dominios y adaptadores --- todo fluye a traves de interfaces.

**Estructura del proyecto:**

```
cmd/
  api/                  # Binario HTTP server (chi router)
  worker/               # Binario NATS consumer (indexacion asincrona)
internal/
  record/               # Dominio: ingesta, validacion, checksum
    record.go           # Entidad Record + IngestRequest
    service.go          # RecordService (orquestador de ingesta)
    validate.go         # Validaciones de dominio
    checksum.go         # SHA-256 determinista
    store.go            # Puerto RecordStore
    errors.go           # Errores tipados del dominio
  indexing/             # Dominio: pipeline de indexacion
    pipeline.go         # Orquestador: fetch -> text -> chunk -> embed -> upsert
    text.go             # Generacion de texto estructurado para embedding
    chunk.go            # Chunker (splitting de texto)
    embed.go            # Puerto Embedder
    index.go            # Puerto IndexStore
    worker.go           # NATS message handler
  query/                # Dominio: busqueda semantica + LLM
    retriever.go        # Puerto Retriever (busqueda vectorial)
    llm.go              # Puerto LanguageModel (completado LLM)
    service.go          # QueryService (RAG + LLM)
    query.go            # QueryRequest / QueryResponse
    context.go          # Ensamblaje de contexto para el LLM
    prompt.go           # System prompt builder
    extract.go          # Extraccion de entidades desde la query
  auth/                 # Dominio: autenticacion por API key
    middleware.go       # Middleware chi para API keys
    apikey.go           # Entidad APIKey
    store.go            # Puerto APIKeyStore
  platform/
    postgres/           # Adaptador: PostgreSQL + pgvector
      db.go             # Pool de conexiones (pgx)
      migrations.go     # golang-migrate runner
      record_repo.go    # Implementa record.RecordStore
      embedding_repo.go # Implementa indexing.IndexStore
      apikey_repo.go    # Implementa auth.APIKeyStore
    nats/               # Adaptador: NATS JetStream
      publisher.go      # Publica record.ingested
      consumer.go       # Consume mensajes del stream
      stream.go         # EnsureStream (crea stream si no existe)
    ollama/             # Adaptador: Ollama (embeddings + LLM local)
      embedder.go       # Implementa indexing.Embedder
      llm.go            # Implementa query.LanguageModel
    anthropic/          # Adaptador: Anthropic Claude
      llm.go            # Implementa query.LanguageModel
    http/               # Adaptador: HTTP handlers
      router.go         # chi router con rutas /api/v1/*
      record_handler.go # CRUD de registros
      search_handler.go # POST /search (RAG sin LLM)
      query_handler.go  # POST /query (RAG + LLM)
      health_handler.go # /health, /health/live, /health/ready
migrations/
  001_records.up.sql          # Tabla records + FTS + indices
  002_record_embeddings.up.sql # Tabla record_embeddings + HNSW + pgvector
  003_api_keys.up.sql          # Tabla api_keys
tests/
  integration/                 # Tests E2E contra infraestructura real
```

**Metricas del codigo:**

| Metrica | Valor |
|---------|-------|
| Archivos fuente Go | 69 |
| Paquetes de dominio | 4 (record, indexing, query, auth) |
| Paquetes de adaptador | 5 (postgres, nats, ollama, anthropic, http) |
| Funciones de test | 119 |
| Binarios | 2 (api, worker) |
| Migraciones SQL | 3 |

---

## 3. Infraestructura

El motor se levanta con **Docker Compose** y 3 servicios de infraestructura + 2 de aplicacion:

| Servicio | Imagen | Proposito |
|----------|--------|-----------|
| PostgreSQL 16 + pgvector | `pgvector/pgvector:pg16` | Almacenamiento relacional, busqueda vectorial (HNSW), full-text search (tsvector spanish) |
| NATS JetStream | `nats:2.10-alpine` | Pipeline asincrono de indexacion. El API publica `record.ingested`, el worker consume |
| Ollama | `ollama/ollama:latest` | Modelo de embeddings local (`nomic-embed-text`, 768 dimensiones). Opcionalmente LLM local |

**Modelo de embeddings:** `nomic-embed-text` genera vectores de 768 dimensiones. Corre localmente en Ollama --- los datos NUNCA salen de la infraestructura para la generacion de embeddings.

**Indice vectorial:** HNSW (Hierarchical Navigable Small World) con `m=16, ef_construction=64`, operador `vector_cosine_ops`. Busqueda aproximada de vecinos mas cercanos con similitud coseno.

---

## 4. API Implementada

Todos los endpoints bajo `/api/v1/*` requieren autenticacion via header `X-API-Key`.

### POST /api/v1/records --- Ingesta

Ingresa un registro operativo al sistema. Genera `record_id`, `checksum` (SHA-256) y `ingested_at` automaticamente. Si NATS esta configurado, publica un evento `record.ingested` para indexacion asincrona.

**Request:**

```json
{
  "occurred_at": "2026-04-05T08:30:00Z",
  "source": "scanner_dock_3",
  "record_type": "scan",
  "entity_ref": "lot:LP-4821",
  "actor_ref": "operator:jperez",
  "title": "Escaneo de ingreso lote LP-4821",
  "payload": {
    "temperature_c": 4.2,
    "humidity_pct": 78,
    "weight_kg": 1250.5,
    "seal_intact": true,
    "dock": "dock_3"
  },
  "tags": ["cold-chain", "incoming"],
  "metadata": {
    "scanner_firmware": "v2.1.4",
    "gps_lat": -33.4489,
    "gps_lon": -70.6693
  }
}
```

**Response (201 Created):**

```json
{
  "record_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "ingested_at": "2026-04-05T08:30:01.234Z",
  "checksum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
}
```

### GET /api/v1/records/{id} --- Recuperar

Retorna el registro completo por `record_id`.

**Response (200 OK):**

```json
{
  "id": "f8a9b0c1-d2e3-4567-89ab-cdef01234567",
  "record_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "occurred_at": "2026-04-05T08:30:00Z",
  "ingested_at": "2026-04-05T08:30:01.234Z",
  "source": "scanner_dock_3",
  "record_type": "scan",
  "entity_ref": "lot:LP-4821",
  "actor_ref": "operator:jperez",
  "title": "Escaneo de ingreso lote LP-4821",
  "payload": {
    "temperature_c": 4.2,
    "humidity_pct": 78,
    "weight_kg": 1250.5,
    "seal_intact": true,
    "dock": "dock_3"
  },
  "object_refs": [],
  "checksum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "tags": ["cold-chain", "incoming"],
  "metadata": {
    "scanner_firmware": "v2.1.4",
    "gps_lat": -33.4489,
    "gps_lon": -70.6693
  }
}
```

### GET /api/v1/records?entity_ref=lot:LP-4821 --- Listar con Filtros

Retorna registros filtrados con paginacion por cursor (keyset pagination).

**Parametros soportados:** `entity_ref`, `actor_ref`, `record_type`, `source`, `tag`, `from`, `to`, `limit` (1-200), `cursor_time` + `cursor_id`.

**Response (200 OK):**

```json
{
  "items": [
    {
      "record_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "source": "scanner_dock_3",
      "record_type": "scan",
      "entity_ref": "lot:LP-4821",
      "occurred_at": "2026-04-05T08:30:00Z",
      "title": "Escaneo de ingreso lote LP-4821"
    }
  ],
  "cursor": {
    "occurred_at": "2026-04-05T08:30:00Z",
    "record_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
  },
  "has_more": false,
  "count": 1
}
```

### POST /api/v1/search --- Busqueda Semantica (RAG sin LLM)

Busqueda directa por similitud coseno sobre los embeddings de los registros. No involucra ningun LLM --- es puramente vectorial.

**Request:**

```json
{
  "q": "lote LP-4821 temperatura",
  "limit": 10
}
```

**Response (200 OK) --- Resultados reales del E2E:**

```json
{
  "results": [
    {
      "record_id": "c3d4e5f6-...",
      "score": 0.739,
      "record_type": "note",
      "occurred_at": "2026-04-05T09:15:00Z",
      "entity_ref": "lot:LP-4821",
      "chunk_text": "[RECORD TYPE: note] ... TITLE: Incidente de temperatura en camara 2 ..."
    },
    {
      "record_id": "a1b2c3d4-...",
      "score": 0.697,
      "record_type": "scan",
      "occurred_at": "2026-04-05T08:30:00Z",
      "entity_ref": "lot:LP-4821",
      "chunk_text": "[RECORD TYPE: scan] ... temperature_c: 4.2 ..."
    },
    {
      "record_id": "b2c3d4e5-...",
      "score": 0.581,
      "record_type": "movement",
      "occurred_at": "2026-04-05T09:00:00Z",
      "entity_ref": "lot:LP-4821",
      "chunk_text": "[RECORD TYPE: movement] ... destination: dock_3 ..."
    }
  ],
  "total": 3,
  "query_ms": 52
}
```

### POST /api/v1/query --- Consulta en Lenguaje Natural (RAG + LLM)

Combina busqueda semantica con un LLM para responder preguntas en lenguaje natural. El motor recupera los registros relevantes, ensambla un contexto, construye un system prompt, y envia todo al LLM configurado.

**Request:**

```json
{
  "q": "Que paso con el lote LP-4821? Hubo algun problema de temperatura?",
  "format": "conversational",
  "entity_scope": "lot:LP-4821",
  "limit_records": 10
}
```

**Response (200 OK) --- Estructura esperada:**

```json
{
  "answer": "El lote LP-4821 ingreso el 5 de abril...",
  "confidence": "high",
  "records_cited": ["a1b2c3d4-...", "c3d4e5f6-..."],
  "gaps": null,
  "suggested_followup": ["Cual fue la accion correctiva?"],
  "retrieved_count": 3,
  "query_ms": 1250
}
```

**Estado:** Documentado e implementado. Requiere configuracion de `LLM_API_KEY` para funcionar. Usa Anthropic Claude por defecto, pero el LLM es agnostico --- la interfaz `LanguageModel` puede ser implementada para cualquier proveedor.

### GET /health --- Salud de Infraestructura

Verifica la conectividad con PostgreSQL, NATS y Ollama. Retorna `200` si todo esta operativo, `503` si algun servicio esta degradado. No requiere autenticacion.

**Response (200 OK):**

```json
{
  "status": "ok",
  "checks": {
    "postgres": "ok",
    "nats": "ok",
    "embeddings": "ok"
  }
}
```

Tambien disponibles: `GET /health/live` (proceso vivo) y `GET /health/ready` (todas las dependencias ok).

---

## 5. Pipeline de Datos (Verificado)

El pipeline de datos fue verificado end-to-end con la infraestructura real. Este es el flujo completo:

1. **Ingesta (HTTP -> PostgreSQL):** `POST /api/v1/records` valida el request, genera `record_id` y `checksum` (SHA-256 sobre los campos canonicos), persiste en PostgreSQL. Latencia: ~90ms.

2. **Publicacion de evento (API -> NATS):** Inmediatamente despues de persistir, el API publica un evento `record.ingested` en el stream `RECORDS` de NATS JetStream con los metadatos del registro (record_id, occurred_at, source, record_type, entity_ref, actor_ref).

3. **Consumo asincrono (NATS -> Worker):** El binario `worker` ejecuta un consumer durable (`indexing-worker`) sobre el subject `record.ingested`. Cada mensaje dispara el pipeline de indexacion.

4. **Pipeline de indexacion (Worker):**
   - Fetch del registro completo desde PostgreSQL
   - Generacion de texto estructurado (`GenerateText`): header con tipo/fuente/fecha, entidad, actor, titulo, payload aplanado con claves ordenadas, tags
   - Chunking del texto (splitting por tamano con overlap)
   - Embedding via Ollama (`nomic-embed-text`, 768 dimensiones)
   - Upsert en tabla `record_embeddings` con embedding + metadatos denormalizados
   - Retry con backoff exponencial (1s, 2s, 4s, 8s, 16s) para errores transitorios

5. **Busqueda semantica (HTTP -> pgvector):** `POST /api/v1/search` embede la query, ejecuta similitud coseno contra el indice HNSW, y retorna los chunks rankeados. Latencia: ~52ms.

---

## 6. Modelo de Privacidad

El motor implementa un modelo de privacidad hibrido con una distincion clara entre procesamiento local y externo:

**Embeddings: LOCAL.** Los embeddings se generan exclusivamente via Ollama corriendo dentro de la infraestructura del usuario. El modelo `nomic-embed-text` (~270MB) corre en CPU o GPU local. Los datos operativos NUNCA salen de la infraestructura para la generacion de embeddings. Esto es una decision de diseno, no una limitacion.

**LLM: EXTERNO, AGNOSTICO.** El motor de consulta en lenguaje natural (`/query`) envia el contexto ensamblado a un LLM externo. Por defecto usa Anthropic Claude, pero la interfaz `LanguageModel` es un puerto del dominio:

```go
type LanguageModel interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
```

Cualquier proveedor puede ser implementado: OpenAI, Mistral, un modelo local via Ollama, o cualquier servicio que acepte un prompt y retorne texto. El usuario elige donde enviar sus datos.

**El motor no impone ningun proveedor de LLM.** La decision de que datos salen de la infraestructura queda en manos del operador del sistema.

---

## 7. Resultados del E2E

Las pruebas de integracion se ejecutaron contra la infraestructura real (Docker Compose) con los 5 servicios levantados.

### Datos ingresados

Se ingresaron 3 registros operativos simulando un flujo real de cadena de frio:

| # | Tipo | Titulo | Entity Ref |
|---|------|--------|------------|
| 1 | `scan` | Escaneo de ingreso lote LP-4821 | `lot:LP-4821` |
| 2 | `movement` | Movimiento lote LP-4821 a dock_3 | `lot:LP-4821` |
| 3 | `note` | Incidente de temperatura en camara 2 | `lot:LP-4821` |

### Indexacion automatica

Los 3 registros fueron indexados automaticamente via el pipeline asincrono:

```
POST record -> PostgreSQL -> NATS (record.ingested) -> Worker -> Ollama (embed) -> pgvector (upsert)
```

No hubo intervencion manual. El worker consumio los 3 eventos y genero los embeddings correctamente.

### Resultados de busqueda semantica

**Query:** `"lote LP-4821 temperatura"`

| Posicion | Tipo | Titulo | Score | Relevancia |
|----------|------|--------|-------|------------|
| 1 | `note` | Incidente de temperatura en camara 2 | **0.739** | Mas relevante: trata directamente sobre un problema de temperatura |
| 2 | `scan` | Escaneo de ingreso lote LP-4821 | **0.697** | Relevante: contiene `temperature_c: 4.2` en el payload |
| 3 | `movement` | Movimiento lote LP-4821 a dock_3 | **0.581** | Menos relevante: solo referencia al lote, no a temperatura |

**Latencia de busqueda:** 52ms

### Analisis de relevancia

El ranking es semanticamente correcto:

- El **incidente de temperatura** (nota) obtuvo el score mas alto porque su titulo y contenido son directamente sobre temperatura.
- El **escaneo** quedo segundo porque su payload contiene un campo de temperatura (`temperature_c: 4.2`), aunque el registro no trata primariamente sobre un problema de temperatura.
- El **movimiento** quedo ultimo porque solo comparte la referencia al lote, pero su contenido (origen/destino/dock) no tiene relacion con temperatura.

Esto demuestra que el modelo `nomic-embed-text` entiende la semantica del contenido, no solo coincidencia lexica.

---

## 8. Bugs Encontrados y Corregidos

Durante el desarrollo y las pruebas E2E se identificaron y corrigieron los siguientes bugs:

1. **NATS: flag `--max_mem_store` inexistente.** La imagen `nats:2.10-alpine` no soporta el flag `--max_mem_store` en linea de comandos. Se removio del `docker-compose.yml`, dejando la configuracion por defecto de JetStream con almacenamiento en disco (`--store_dir=/data`).

2. **PostgreSQL: NOT NULL violation por payload/tags nil.** Cuando un registro se ingresaba sin `payload` o `tags`, el INSERT fallaba porque las columnas tienen constraint `NOT NULL DEFAULT '{}'`. Se agregaron defaults en la capa de servicio: si `payload` es nil se inicializa como `map[string]any{}`, si `tags` es nil se inicializa como `[]string{}`.

3. **Record handler: errores tragados sin logging.** Varios paths de error en el handler de records retornaban 500 sin loguear el error real. Se agrego `slog.Error` en cada caso para facilitar diagnostico en produccion.

---

## 9. Pendiente

Prioridad de implementacion:

1. **Test /query con Claude API key.** Ejecutar el E2E completo con el LLM para verificar el flujo RAG + generacion de respuesta en lenguaje natural.

2. **Dockerfiles para api/worker.** Crear imagenes Docker para ambos binarios. Actualmente se ejecutan con `go run`.

3. **Import CSV masivo.** Endpoint para importar registros operativos desde archivos CSV. Necesario para migraciones de datos historicos.

4. **OCR para fotos.** Procesar imagenes adjuntas (fotos de etiquetas, guias de despacho) con OCR para extraer texto indexable.

5. **Busqueda hibrida (vector + full-text).** Combinar la busqueda vectorial (coseno sobre embeddings) con la busqueda full-text (tsvector spanish) ya configurada en PostgreSQL. La tabla `records` ya tiene la columna `search_vector` generada.

6. **Streaming de respuestas (SSE).** Server-Sent Events para el endpoint `/query`, permitiendo que la respuesta del LLM se transmita progresivamente al cliente.

7. **Memoria de sesion para consultas multi-turno.** Mantener contexto entre consultas sucesivas del mismo usuario para permitir conversaciones (e.g., "y que paso despues?").

8. **Evidencia de integridad por cadena de hashes.** Cada registro tiene un `checksum` individual. El paso siguiente es encadenar los checksums para crear una cadena de integridad que detecte alteraciones retroactivas.

---

## 10. Conclusion

El motor funciona. La tesis esta probada: registros operativos de cualquier tipo pueden ser ingeridos, indexados semanticamente y consultados por similitud vectorial con resultados relevantes y rankeados correctamente.

La arquitectura es limpia (hexagonal, con puertos e interfaces), esta testeada (119 funciones de test), y es extensible (agregar un nuevo proveedor de LLM o un nuevo tipo de registro no requiere cambios en el dominio).

Lo que queda por hacer es endurecimiento y expansion de funcionalidades, no cambios fundamentales. El nucleo del motor --- ingesta, indexacion, busqueda semantica --- esta operativo y verificado con datos reales.

> *"La realidad, cuando es correctamente registrada, puede ser consultada como si fuera un sistema."*

---

## Aviso de Publicacion Defensiva

Este documento constituye una publicacion defensiva (defensive publication) del sistema "Trazabilidad Operativa Inteligente", con fecha de creacion verificable en el historial de control de versiones del repositorio.

El sistema descrito --- incluyendo su arquitectura, pipeline de datos, modelo de privacidad hibrido (embeddings locales + LLM agnostico externo), y la combinacion especifica de PostgreSQL/pgvector + NATS JetStream + Ollama para trazabilidad operativa con busqueda semantica --- constituye arte previo (prior art) a partir de la fecha de este commit.

**Autor:** Ricardo Agurto  
**Fecha:** Abril 2026  
**Licencia:** Apache License 2.0  
**Repositorio:** github.com/trazabilidad-operativa-inteligente/platform
