# URL Shortener API — Feature Specification

## Overview

Build a URL shortener HTTP service in Go. Users can create short URLs, get redirected,
view click statistics, and manage their links. All storage is in-memory.

## API Endpoints

### POST /api/shorten

Create a shortened URL.

**Request body:**
```json
{"url": "https://example.com", "created_by": "user@example.com"}
```

**Response (201 Created):**
```json
{
  "short_code": "Ab3kX9pL",
  "short_url": "http://localhost:8080/r/Ab3kX9pL",
  "original_url": "https://example.com"
}
```

**Validation rules:**
- `url` must not be empty → 400 `{"error": "url is required"}`
- `url` must start with `http://` or `https://` → 400 `{"error": "invalid url"}`
- `url` must parse via `net/url.Parse` without error → 400 `{"error": "invalid url"}`
- Malformed JSON → 400 `{"error": "invalid request body"}`

**Deduplication:**
If the exact `url` already exists (and is not deleted), return the existing entry
instead of creating a new one. Still return 201.

### GET /r/{code}

Redirect to the original URL.

- **301 Moved Permanently** with `Location` header set to the original URL
- Increment the click counter for this URL
- **404** if the code doesn't exist or was deleted

### GET /api/urls/{code}

Get statistics for a shortened URL.

**Response (200 OK):**
```json
{
  "short_code": "Ab3kX9pL",
  "original_url": "https://example.com",
  "created_at": "2024-01-15T10:30:00Z",
  "clicks": 42,
  "created_by": "user@example.com"
}
```

- **404** if not found or deleted

### GET /api/urls

List all shortened URLs (paginated).

**Query parameters:**
- `page` (default: 1)
- `per_page` (default: 20)

**Response (200 OK):**
```json
{
  "urls": [...],
  "total": 100,
  "page": 1,
  "per_page": 20
}
```

- `urls` must be an empty JSON array `[]` (not null) when there are no results
- Deleted URLs must NOT appear in the list

### DELETE /api/urls/{code}

Soft-delete a URL. The URL remains in storage but is no longer accessible.

- **204 No Content** on success
- **404** if not found

## Rate Limiting

The `POST /api/shorten` endpoint must be rate-limited:
- Maximum N requests per IP address per time window
- When exceeded, return **429 Too Many Requests**: `{"error": "rate limit exceeded"}`
- IP is extracted from `RemoteAddr` — you must strip the port (use `net.SplitHostPort`)

## Short Code Generation

- Exactly 8 characters
- Alphanumeric only: `a-z`, `A-Z`, `0-9`
- Randomly generated (use `crypto/rand` or `math/rand`)

## Data Model

```go
type URL struct {
    ShortCode   string    `json:"short_code"`
    OriginalURL string    `json:"original_url"`
    CreatedAt   time.Time `json:"created_at"`
    Clicks      int       `json:"clicks"`
    CreatedBy   string    `json:"created_by,omitempty"`
    Deleted     bool      `json:"-"`
}
```

## Concurrency

The store MUST be safe for concurrent access. Use `sync.RWMutex` with appropriate
read/write locking. One test fires 100 concurrent goroutines — race-unsafe code will fail.

## Files to Implement

| File | What to do |
|------|------------|
| `store.go` | Implement all methods on `MemoryStore` |
| `handler.go` | Implement all handler methods + `GenerateCode()` |
| `middleware.go` | Implement `RateLimiter` struct, constructor, and `Middleware()` |

**Do NOT modify:** `main.go`, `store_test.go`, `handler_test.go`
