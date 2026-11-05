# Architecture and engineering decisions

## System boundary

The application is a local-first web app. The browser handles microphone capture and presentation. The Go server owns the API key, command execution, persistence and calls to Gnani.

```text
Browser
  ├─ microphone capture → 16 kHz WAV
  ├─ dashboard, charts and command UI
  └─ audio playback
        │
        ▼
Go HTTP server
  ├─ /api/stt → Gnani Prisma
  ├─ intent + entity parser
  ├─ inventory transaction engine
  ├─ sales and reorder analytics
  ├─ JSON persistence
  └─ /api/tts → Gnani Timbre
```

## Why the API key is not in JavaScript

Putting a long-lived key in frontend JavaScript exposes it to every visitor and browser extension. Vyapari sends audio to its own Go endpoint. The server adds `X-API-Key-ID` only when communicating with Gnani.

## Audio pipeline

Chrome commonly records `audio/webm`, while the Gnani REST documentation accepts WAV, MP3, OGG, FLAC, AAC and M4A. To avoid FFmpeg and format incompatibility, the app captures raw PCM through the Web Audio API, downsamples it to 16 kHz mono and writes a valid 16-bit WAV in the browser.

## Speech recognition strategy

The default request uses:

```text
model=prisma-v3.0
language_code=indic-auto
format=codemix
```

The request includes the current product names as keyterms. If the account/API rejects Prisma v3.0 with a validation response, the server retries once with:

```text
model=prisma-v2.5
language_code=te-IN
format=transcribe
```

Calls are serialised to reduce accidental `429 RATE_LIMITED` responses on low-concurrency programme accounts. Retryable 429/5xx responses use short bounded backoff.

## Intent engine

The intent engine is deterministic rather than an unrestricted LLM. This is deliberate for inventory operations:

- A sale must not happen because a generative model hallucinated an action.
- Product and quantity extraction is inspectable and unit-tested.
- The engine recognises English, Telugu script and common transliterated Telugu words.
- Product aliases are editable in the add-product form.
- Unsupported commands fail safely and suggest examples.

Supported intentions include stock check, inventory list, sale, restock, low-stock list, sales summary, profit summary, reorder plan, top products, business briefing and undo.

## Persistence

State is stored in `data/store.json`. Mutations are protected by a `sync.RWMutex`, copied before modification and rolled back if persistence fails. The new state is written to a temporary file and then replaces the prior state.

This keeps the challenge build dependency-free. A production version would use PostgreSQL with transaction isolation, authentication, tenant separation and audit retention.

## Reorder logic

For each product, the engine calculates seven-day average unit demand and estimated days of stock cover. A recommendation is created when stock is below its configured threshold or has fewer than five days of cover.

The target is the larger of:

- the configured target stock; and
- approximately fourteen days of recent demand.

Recommendations include urgency, quantity, cost and an explanation. This is an operational heuristic, not a financial forecast.

## Failure modes

- **No API key:** typed commands and dashboard remain available; voice endpoints return a clear configuration message.
- **Microphone blocked:** users can type the same command.
- **Gnani STT/TTS unavailable:** the UI displays the API error; TTS can fall back to browser speech synthesis.
- **Insufficient stock:** the sale is rejected without changing data.
- **Repeated click:** the speech client serialises external calls.
- **Corrupt data file:** the file is backed up with a `.broken-*` suffix and synthetic demo data is regenerated.

## Production extensions

- PostgreSQL and transactional outbox
- User authentication and shop tenancy
- Barcode ingestion
- GST invoice integration
- Supplier ordering and WhatsApp confirmation
- Streaming STT/TTS over WebSocket or SSE
- Mobile app wrapper
- Forecast evaluation against real historical demand
- Human approval for high-value inventory actions
