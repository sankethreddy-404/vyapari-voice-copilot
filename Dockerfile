FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/vyapari ./cmd/server

FROM alpine:3.20
RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=builder /out/vyapari /app/vyapari
COPY web /app/web
RUN mkdir -p /app/data && chown -R app:app /app
USER app
EXPOSE 8080
ENV APP_ADDR=:8080 DATA_FILE=/app/data/store.json
VOLUME ["/app/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD wget -q -O - http://127.0.0.1:8080/api/health || exit 1
CMD ["/app/vyapari"]
