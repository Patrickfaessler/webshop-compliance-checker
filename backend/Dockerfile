# Go API + scan worker with headless Chromium (ADR-4). Build context: repository root.
# Spec: specs/07-architecture.md (DEP-1).
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends chromium fonts-liberation fonts-dejavu-core ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --create-home --uid 10001 checker
COPY --from=build /out/api /usr/local/bin/api
USER checker
WORKDIR /home/checker
# No sandbox: the container is the isolation boundary (specs/05-crawler.md); SSRF filtering stays on.
ENV API_ADDR=:8080 CHROME_PATH=/usr/bin/chromium CHROME_NO_SANDBOX=1 STORE=memory
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
