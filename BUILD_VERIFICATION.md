# Build Verification

Verified on November 4, 2026 in a clean Linux container.

## Checks passed

```text
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
go build -trimpath -o /tmp/vyapari ./cmd/server
node --check web/app.js
node --check web/service-worker.js
```

## Runtime smoke test passed

- HTTP server started successfully.
- `/api/health` returned HTTP 200.
- `/api/dashboard` returned the seeded shop dashboard with nine products.
- A Telugu-English sale command updated stock and sales correctly.
- Undo restored the previous state correctly.
- Static HTML, CSS, JavaScript, PWA manifest, and service worker were served.
- Desktop and mobile layouts rendered without browser-console errors.

## External integration boundary

The application is compiled and runtime-tested without an API key. Live Gnani transcription and speech synthesis require:

- a valid `GNANI_API_KEY` in `.env`;
- available Gnani programme credits;
- internet access;
- the current Gnani API service being available.

The project uses the official Gnani endpoint formats documented when this build was prepared. If Gnani later changes an endpoint or field, update `internal/gnani/client.go` using the current dashboard documentation.
