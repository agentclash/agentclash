#!/bin/bash
set -euo pipefail
# Docker uses the iptables backend on the reviewed AL2023 host. Refuse to proceed
# if its enforcement chain is absent. Must run after every Docker daemon restart.
iptables -w -n -L DOCKER-USER >/dev/null
for net in 172.30.72.0/24 172.30.73.0/24; do
  if ! iptables -w -C DOCKER-USER -s "$net" -d 169.254.169.254/32 -j REJECT 2>/dev/null; then
    iptables -w -I DOCKER-USER 1 -s "$net" -d 169.254.169.254/32 -j REJECT
  fi
done
