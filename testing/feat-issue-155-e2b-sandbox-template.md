# feat/issue-155-e2b-sandbox-template — Test Contract

## Functional Behavior

### E2B Template Expansion
- Template uses Ubuntu 24.04 base image
- Template runs as root user (not `user`)
- All of the following are pre-installed and available on PATH:
  - Python 3.11+ with pip
  - Node.js 20+ with npm
  - Go 1.22+
  - gcc, g++, make, cmake
  - pdftotext (poppler-utils), pandoc
  - jq, csvtool, xmllint (libxml2-utils), sqlite3
  - curl, wget
  - ripgrep (rg), fd-find (fd/fdfind)
- Python packages installed: PyPDF2, pdfplumber, requests, httpx, csvkit

### Helper Scripts
- `/tools/pdf_extract.py` exists, is executable, accepts `<file_path> [--page N]`
- `/tools/csv_to_json.py` exists, is executable, accepts `<file_path> [--orient records|columns]`
- `/tools/json_query.py` exists, is executable, accepts `<file_path> <path>`
- All scripts exit 0 with `--help` flag

### Per-Pack Sandbox Configuration
- Challenge pack YAML `version.sandbox` block is parsed into `SandboxConfig` struct:
  - `network_access: bool` (default false)
  - `network_allowlist: []string` (CIDR notation)
  - `env_vars: map[string]string` (literal key-value pairs)
  - `additional_packages: []string` (apt package names)
- Validation rejects:
  - Invalid CIDR in `network_allowlist` (e.g., `"not-a-cidr"`)
  - Invalid apt package names in `additional_packages` (e.g., `"; rm -rf /"`)
  - Invalid env var keys (e.g., `"123BAD"`, `"FOO BAR"`)
- Valid sandbox config passes validation and round-trips through YAML -> Go struct -> JSON

### Sandbox CreateRequest Propagation
- `sandbox.CreateRequest` carries: `EnvVars`, `NetworkAllowlist`, `AdditionalPackages`, `TemplateID`
- Native executor reads `version.sandbox` from challenge pack manifest and populates these fields
- E2B provider passes `envVars` and `network.allowOut` to the E2B API at sandbox creation
- E2B provider runs `apt-get install` post-boot when `AdditionalPackages` is non-empty
- E2B provider uses `request.TemplateID` if set, otherwise falls back to `config.TemplateID`

### Default Sandbox User
- `defaultSandboxUser` constant changed from `"user"` to `"root"`
- File operations and gRPC auth use `"root"` as the username

### Template Versioning
- Build scripts produce `agentclash-v2-dev` and `agentclash-v2` template names
- Challenge pack manifest can carry a `sandbox_template_id` field for pinning

---

## Unit Tests

### Challenge Pack Parsing (`backend/internal/challengepack/`)
- `TestParseSandboxConfig_Defaults` — empty sandbox block yields zero-value SandboxConfig
- `TestParseSandboxConfig_AllFields` — all fields populated from YAML
- `TestValidateSandboxConfig_ValidCIDR` — `["10.0.0.0/8", "192.168.1.0/24"]` passes
- `TestValidateSandboxConfig_InvalidCIDR` — `["not-a-cidr"]` returns error
- `TestValidateSandboxConfig_ValidPackages` — `["ffmpeg", "tesseract-ocr"]` passes
- `TestValidateSandboxConfig_InvalidPackages` — `["; rm -rf /"]` returns error
- `TestValidateSandboxConfig_ValidEnvVars` — `{"FOO": "bar", "DB_URL": "..."}` passes
- `TestValidateSandboxConfig_InvalidEnvVarKey` — `{"123BAD": "x"}` returns error

### Native Executor Sandbox Policy (`backend/internal/engine/`)
- `TestApplySandboxConfig_WithSandboxBlock` — manifest with sandbox block populates EnvVars, NetworkAllowlist, AdditionalPackages on CreateRequest
- `TestApplySandboxConfig_WithoutSandboxBlock` — manifest without sandbox block leaves new fields empty (backward compat)
- `TestApplySandboxConfig_TemplateIDFromSandboxOnly` — sandbox-level template ID applied when no version-level pin
- `TestApplySandboxConfig_InvalidJSON` — gracefully handles malformed manifest

### E2B Client (`backend/internal/sandbox/e2b/`)
- `TestCreateSandboxRequestEnvVars` — envVars field serializes correctly in JSON
- `TestCreateSandboxRequestNetwork` — network.allowOut field serializes correctly
- `TestCreateSandboxRequestNoOptionalFields` — omitempty works, no null fields in JSON

## Integration / Functional Tests
N/A — E2B sandbox integration requires live API key. Covered by smoke tests.

## Smoke Tests (`backend/internal/sandbox/e2b/smoke_test.go`, build tag `e2bsmoke`)
- Sandbox creates successfully with new v2 template
- `python3 --version` returns 3.x
- `node --version` returns v20.x+
- `go version` returns go1.22+
- `jq --version` returns non-empty
- `sqlite3 --version` returns non-empty
- `python3 /tools/pdf_extract.py --help` exits 0
- `python3 /tools/csv_to_json.py --help` exits 0
- `python3 /tools/json_query.py --help` exits 0
- File operations work as root user
- Exec commands run as root

## E2E Tests
N/A — Full E2E requires Temporal workflow + run creation. Out of scope for this change.

## Manual / cURL Tests

### Verify template builds
```bash
cd backend/e2b-template
npx tsx build.dev.ts
# Expected: template builds successfully, prints template ID
```

### Verify Go compilation
```bash
cd backend
go vet ./...
# Expected: no errors
```

### Verify unit tests pass
```bash
cd backend
go test ./internal/challengepack/... -v -run TestSandbox
go test ./internal/engine/... -v -run TestApplyChallengeSandboxPolicy
go test ./internal/sandbox/e2b/... -v -run TestCreateSandboxRequest
# Expected: all pass
```

### Verify smoke tests (requires E2B_API_KEY)
```bash
cd backend
E2B_API_KEY=<key> E2B_TEMPLATE_ID=agentclash-v2-dev \
  go test -tags e2bsmoke ./internal/sandbox/e2b/... -v -timeout 120s
# Expected: all pass
```
