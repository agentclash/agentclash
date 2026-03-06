FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api-server ./cmd/api-server

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /api-server /api-server
COPY migrations /migrations
EXPOSE 8080
ENTRYPOINT ["/api-server"]
