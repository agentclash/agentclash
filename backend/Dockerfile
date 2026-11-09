# Build context is the REPO ROOT (not backend/): the backend module depends on
# the sibling runtime module via `replace ../runtime` in backend/go.mod, so both
# module directories must be inside the build context.
#
#   docker build -f backend/Dockerfile --build-arg TARGET=api-server .
#
# On Railway, leave the service Root Directory unset and point Dockerfile Path
# at backend/Dockerfile.

# ---- Build stage ----
FROM golang:1.26.10-alpine3.24@sha256:87c65a688975c0b1d4585d204874c45ca78354670e3a5f11368c5a492303d696 AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY backend/go.mod backend/go.sum backend/
COPY runtime/go.mod runtime/go.sum runtime/

WORKDIR /src/backend
RUN go mod download

COPY backend /src/backend
COPY runtime /src/runtime

# TARGET selects which binary to build: api-server or worker
ARG TARGET=api-server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app ./cmd/${TARGET}
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /migrator ./cmd/db-migrate

# ---- Runtime stage ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata postgresql17-client bash

COPY --from=builder /app /app
COPY --from=builder /migrator /migrator

# Include migrations so Railway deploy commands can run them
COPY --from=builder /src/backend/db/migrations /migrations
COPY --from=builder /src/backend/scripts/migrate.sh /migrate.sh
RUN chmod +x /migrate.sh

EXPOSE 8080
ENTRYPOINT ["/app"]
