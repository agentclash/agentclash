package inputs

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Parser receives bytes, never credentials or storage URLs.
type Parser interface {
	Extract(context.Context, []byte) (Extraction, error)
}

//go:embed parser.py
var parserScript []byte

// IsolatedParser requires a pinned Python virtualenv containing pdfplumber.
// Bubblewrap supplies a private filesystem, PID/user/network namespace and
// empty environment. Python applies resource limits before loading the parser.
type IsolatedParser struct {
	Runtime  string
	Launcher string
}

func NewParser(ctx context.Context, runtime string) (*IsolatedParser, error) {
	if os.Geteuid() == 0 {
		return nil, errors.New("PDF reader must run under an unprivileged worker account")
	}
	if runtime == "" {
		return nil, errors.New("PDF runtime is not configured")
	}
	runtime, err := filepath.Abs(runtime)
	if err != nil {
		return nil, err
	}
	launcher, err := exec.LookPath("bwrap")
	if err != nil {
		return nil, err
	}
	p := &IsolatedParser{Runtime: runtime, Launcher: launcher}
	out, err := p.run(ctx, nil, true)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(out)) != "pdfplumber-0.11.7/v1" {
		return nil, errors.New("PDF isolation self-check failed")
	}
	return p, nil
}

type limitedBuffer struct {
	bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.max {
		return 0, errors.New("parser output limit exceeded")
	}
	return b.Buffer.Write(p)
}
func (p *IsolatedParser) run(ctx context.Context, data []byte, check bool) ([]byte, error) {
	return p.runScript(ctx, data, check, parserScript)
}
func (p *IsolatedParser) runScript(ctx context.Context, data []byte, check bool, script []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "vibe-pdf-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if err = os.WriteFile(filepath.Join(dir, "parser.py"), script, 0600); err != nil {
		return nil, err
	}
	args := []string{"--unshare-all", "--die-with-parent", "--new-session", "--cap-drop", "ALL", "--clearenv", "--ro-bind", "/usr", "/usr", "--ro-bind", "/lib", "/lib"}
	if _, e := os.Stat("/lib64"); e == nil {
		args = append(args, "--ro-bind", "/lib64", "/lib64")
	}
	args = append(args, "--proc", "/proc", "--dev", "/dev", "--dir", "/tmp", "--ro-bind", p.Runtime, "/runtime", "--ro-bind", filepath.Join(dir, "parser.py"), "/parser.py", "--setenv", "PYTHONDONTWRITEBYTECODE", "1", "--setenv", "PYTHONNOUSERSITE", "1", "--setenv", "PATH", "/usr/bin", "--chdir", "/tmp")
	if !check {
		if err = os.WriteFile(filepath.Join(dir, "input.pdf"), data, 0600); err != nil {
			return nil, err
		}
		args = append(args, "--ro-bind", filepath.Join(dir, "input.pdf"), "/input.pdf")
	}
	// Root filesystem becomes read-only after constructing the namespace. No
	// writable temporary filesystem is needed by this text-only extraction.
	args = append(args, "--remount-ro", "/", "/runtime/bin/python", "-I", "/parser.py")
	if check {
		args = append(args, "--check")
	}
	cmd := exec.CommandContext(ctx, p.Launcher, args...)
	cmd.Env = []string{}
	cmd.WaitDelay = time.Second
	out := &limitedBuffer{max: 1_000_000}
	stderr := &limitedBuffer{max: 4096}
	cmd.Stdout = out
	cmd.Stderr = stderr
	if err = cmd.Run(); err != nil {
		return nil, fmt.Errorf("isolated PDF reader failed: %w", err)
	}
	return out.Bytes(), nil
}
func (p *IsolatedParser) Extract(ctx context.Context, data []byte) (Extraction, error) {
	out, err := p.run(ctx, data, false)
	if err != nil {
		return Extraction{}, errors.New("The PDF reader could not finish. Use a smaller PDF or paste its text instead.")
	}
	var receipt struct {
		Extraction
		Error string `json:"error"`
	}
	if err = json.Unmarshal(out, &receipt); err != nil {
		return Extraction{}, errors.New("The PDF reader returned an invalid result. Paste the text instead.")
	}
	if receipt.Error != "" {
		return Extraction{}, errors.New(receipt.Error)
	}
	return receipt.Extraction, ValidateExtraction(receipt.Extraction)
}
