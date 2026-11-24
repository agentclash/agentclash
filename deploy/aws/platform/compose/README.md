# Maintained Compose binary

Build Docker Compose v5.5.1 from its checksum-verified public Go module with the
scanned Go SDK, containerd v2.3.7 and the fixed network dependency. The command
version and module version remain v5.5.1. The official binary is a pinned
remediation input. The bootstrap uses the rebuilt binary with its own SHA256
pin. The release checks compare the original findings with the full bootstrap
scan. The installed tool includes dependency notices.
