#!/bin/sh
set -eu
# Membership addresses must be actual current container IPs, not DNS aliases.
# The JSON template has exactly one sentinel in membership.broadcastAddress.
ip=$(hostname -i | awk '{print $1}')
case "$ip" in *[!0-9.]*|'') exit 1 ;; esac
sed 's/"broadcastAddress": "__CONTAINER_IP__"/"broadcastAddress": "'"$ip"'"/' /run/secrets/production.template.json > /tmp/production.yaml
# The pinned server seeds from heartbeats younger than 20 seconds. After a
# single-host stop, let old addresses age out before all roles bootstrap together.
# This is a bounded startup delay, never a deletion of membership/workflow state.
sleep 25
exec temporal-server --root / --config tmp --env production start --service frontend --service history --service matching --service worker
