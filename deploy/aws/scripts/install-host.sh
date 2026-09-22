#!/bin/bash
set -euo pipefail
umask 077
# Invoked from the hash-verified bootstrap bundle, before private host configuration.
# Step 9 packages the reviewed Compose binary with its SHA256SUMS file.
cd /opt/agentclash
sha256sum --check tools/SHA256SUMS
install -d -m 755 /usr/local/lib/docker/cli-plugins
install -m 755 tools/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose
install -d -m 700 /var/lib/agentclash /etc/agentclash
touch /var/lib/agentclash/recovery-unverified
install -d -m 755 /var/lib/agentclash/caddy /run/agentclash
chown 10001:10001 /var/lib/agentclash/caddy
install -m 644 deploy/aws/systemd/agentclash-*.service deploy/aws/systemd/agentclash-*.timer /etc/systemd/system/
install -m 644 deploy/aws/systemd/run-agentclash.mount /etc/systemd/system/
# Conservative host limits. Services are started only by the approved operator flow.
install -d -m 755 /etc/systemd/journald.conf.d
cat > /etc/systemd/journald.conf.d/agentclash.conf <<'JOURNAL'
[Journal]
SystemMaxUse=200M
RuntimeMaxUse=50M
MaxRetentionSec=14day
JOURNAL
systemctl daemon-reload
systemctl enable --now run-agentclash.mount agentclash-firewall.service
systemctl enable agentclash-monitor.timer agentclash-backup.timer
# Do not format/mount an unidentified disk, load secrets or start application writers.
