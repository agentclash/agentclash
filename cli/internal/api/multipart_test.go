package api

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPostMultipartSendsContentLengthAndSupportsGetBody ensures that the
// request body is spooled (not streamed via io.Pipe), so Go sends a real
// Content-Length header instead of Transfer-Encoding: chunked — some
// gateways / WAFs reject chunked uploads with 411 / 400 — and exposes a
// GetBody so same-origin 307/308 redirects can replay the body.
func TestPostMultipartSendsContentLengthAndSupportsGetBody(t *testing.T) {
	var gotContentLength int64
	var gotTransferEncoding string
	var gotFormField string
	var gotFileContents string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentLength = r.ContentLength
		gotTransferEncoding = strings.Join(r.TransferEncoding, ",")

		ct, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if ct != "multipart/form-data" {
			t.Fatalf("Content-Type = %q, want multipart/form-data", ct)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("reading part: %v", err)
			}
			body, _ := io.ReadAll(p)
			switch p.FormName() {
			case "note":
				gotFormField = string(body)
			case "blob":
				gotFileContents = string(body)
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "")
	payload := strings.Repeat("0123456789", 10_000) // 100 KB, well beyond a header line
	resp, err := client.PostMultipart(context.Background(), "/upload",
		map[string]string{"note": "hello"},
		map[string]FileUpload{
			"blob": {Filename: "data.bin", Reader: strings.NewReader(payload)},
		},
	)
	if err != nil {
		t.Fatalf("PostMultipart: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, resp.Body)
	}

	if gotTransferEncoding != "" {
		t.Fatalf("request used Transfer-Encoding %q, want empty (needs Content-Length)", gotTransferEncoding)
	}
	if gotContentLength <= 0 {
		t.Fatalf("request missing Content-Length, got %d", gotContentLength)
	}
	if gotFormField != "hello" {
		t.Fatalf("form field = %q, want hello", gotFormField)
	}
	if gotFileContents != payload {
		t.Fatalf("file contents corrupted: len got=%d want=%d", len(gotFileContents), len(payload))
	}
}
