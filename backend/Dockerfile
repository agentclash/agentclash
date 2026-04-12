# ---- Build stage ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# TARGET selects which binary to build: api-server or worker
ARG TARGET=api-server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app ./cmd/${TARGET}

# ---- Runtime stage ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata postgresql17-client bash

COPY --from=builder /app /app

# Include migrations so Railway deploy commands can run them
COPY --from=builder /src/db/migrations /migrations
COPY --from=builder /src/scripts/migrate.sh /migrate.sh
RUN chmod +x /migrate.sh

EXPOSE 8080
ENTRYPOINT ["/app"]
