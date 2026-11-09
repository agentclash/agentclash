"""Run only through the isolated launcher. stdout is a bounded JSON receipt."""
import json
import resource
import sys

resource.setrlimit(resource.RLIMIT_AS, (512 * 1024 * 1024,) * 2)
resource.setrlimit(resource.RLIMIT_CPU, (15, 15))
resource.setrlimit(resource.RLIMIT_FSIZE, (1024 * 1024,) * 2)
resource.setrlimit(resource.RLIMIT_NOFILE, (32, 32))
resource.setrlimit(resource.RLIMIT_NPROC, (1, 1))
resource.setrlimit(resource.RLIMIT_CORE, (0, 0))

import pdfplumber

VERSION = "pdfplumber-0.11.10/v1"
if pdfplumber.__version__ != "0.11.10":
    raise RuntimeError("Unexpected PDF runtime")
if sys.argv[-1] == "--check":
    print(VERSION)
    sys.exit(0)

try:
    pages, warnings = [], []
    count = 0
    with pdfplumber.open("/input.pdf") as pdf:
        if not 1 <= len(pdf.pages) <= 30:
            raise ValueError("Use a PDF with 1 to 30 pages.")
        for number, page in enumerate(pdf.pages, 1):
            text = page.extract_text() or ""
            count += len(text)
            if count > 60000:
                raise ValueError("This PDF contains too much text. Use a smaller document or paste a shorter section; nothing was truncated.")
            if page.images:
                warnings.append(f"Page {number} contains images that were not read. Only extracted text will be used.")
            pages.append({"number": number, "text": text})
            page.close()
    print(json.dumps({"pages": pages, "warnings": warnings, "version": VERSION}))
except ValueError as error:
    print(json.dumps({"error": str(error)}))
except Exception:
    print(json.dumps({"error": "This PDF could not be read. Paste its text instead. Scanned, encrypted and damaged PDFs are not supported."}))
