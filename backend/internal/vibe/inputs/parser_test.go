package inputs

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
)

// A tiny valid text PDF keeps the reader test independent of a second parser.
func textPDF(pages int) []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", ""}
	kids := []string{}
	for i := 0; i < pages; i++ {
		page, content := len(objects)+1, len(objects)+2
		kids = append(kids, fmt.Sprintf("%d 0 R", page))
		stream := "BT /F1 12 Tf 20 100 Td (Invoice total 42.00) Tj ET"
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200] /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> /Contents %d 0 R >>", content), fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}
	objects[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), pages)
	var out strings.Builder
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return []byte(out.String())
}

func TestIsolatedPDFReader(t *testing.T) {
	runtime := os.Getenv("VIBE_TEST_PDF_RUNTIME")
	if runtime == "" {
		t.Skip("set VIBE_TEST_PDF_RUNTIME to exercise the real isolated reader")
	}
	p, err := NewParser(context.Background(), runtime)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.Extract(context.Background(), textPDF(2))
	if err != nil || len(result.Pages) != 2 || !strings.Contains(result.Pages[1].Text, "Invoice total 42.00") {
		t.Fatalf("extraction: %+v %v", result, err)
	}
	for name, data := range map[string][]byte{"too_many_pages": textPDF(31), "corrupt": []byte("%PDF-not a document")} {
		t.Run(name, func(t *testing.T) {
			if _, err := p.Extract(context.Background(), data); err == nil {
				t.Fatal("unreadable document was accepted")
			}
		})
	}
}

func TestTextBoundsDoNotTruncate(t *testing.T) {
	if err := ValidateText(strings.Repeat("界", MaxTextCharacters)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateText(strings.Repeat("界", MaxTextCharacters+1)); err == nil {
		t.Fatal("oversized text accepted")
	}
	for _, text := range []string{"", "\x00", string([]byte{0xff})} {
		if ValidateText(text) == nil {
			t.Fatal("invalid text accepted")
		}
	}
	if ValidateExtraction(Extraction{Pages: []Page{{1, ""}}}) == nil {
		t.Fatal("image-only PDF accepted")
	}
	if ValidateExtraction(Extraction{Pages: []Page{{2, "text"}}}) == nil {
		t.Fatal("invalid page mapping accepted")
	}
}

func TestReaderIsolation(t *testing.T) {
	runtime := os.Getenv("VIBE_TEST_PDF_RUNTIME")
	if runtime == "" {
		t.Skip("real isolation probe requires VIBE_TEST_PDF_RUNTIME")
	}
	p, err := NewParser(context.Background(), runtime)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("VIBE_SECRET_PROBE", "must-not-escape")
	prelude := strings.Split(string(parserScript), "import pdfplumber")[0]
	probe := fmt.Sprintf(`
import os, socket
assert 'VIBE_SECRET_PROBE' not in os.environ
assert not os.path.exists('/home') and not os.path.exists('/app')
assert not os.path.exists('/etc/resolv.conf')
assert os.geteuid() != 0
s=socket.socket(); s.settimeout(.2)
try:
    s.connect(('127.0.0.1', %d))
except OSError:
    pass
else:
    raise AssertionError('host network reachable')
try:
    open('/tmp/escape','w')
except OSError:
    pass
else:
    raise AssertionError('writable sandbox root')
try:
    allocation=bytearray(600*1024*1024)
except MemoryError:
    pass
else:
    raise AssertionError('memory limit missing')
assert resource.getrlimit(resource.RLIMIT_CPU)[0] == 15
assert resource.getrlimit(resource.RLIMIT_NPROC)[0] == 1
print('isolated')
`, listener.Addr().(*net.TCPAddr).Port)
	result, err := p.runScript(context.Background(), nil, true, []byte(prelude+probe))
	if err != nil || strings.TrimSpace(string(result)) != "isolated" {
		t.Fatalf("isolation: %s %v", result, err)
	}
}
