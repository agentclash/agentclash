# Maintained Compose binary

Build Docker Compose v5.5.1 from its checksum-verified public Go module, using
the scanned Go SDK and a fixed network dependency. The command version and module
version remain v5.5.1. The official binary remains a pinned remediation input;
the bootstrap consumes only the rebuilt binary with its separately reviewed
SHA256. Original findings are retained and reconciled against the strict full
bootstrap scan. Dependency notices accompany the installed tool.
