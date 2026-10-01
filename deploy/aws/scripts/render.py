"""Render private runtime configuration from validated Secrets Manager payloads."""

import hashlib
import json
from pathlib import Path
import re

from common import require, write_private
from secret_store import UIDS

ROOT = Path(__file__).resolve().parents[1]


def temporal_config(env):
    host = env["DB_HOST"]
    require(re.fullmatch(r"[a-zA-Z0-9.-]+", host), "Invalid database hostname")
    stores = {}
    for key, database, user, password, cap in [
        ("default", "temporal", "temporal_runtime", env["DB_PASSWORD"], 5),
        (
            "visibility",
            "temporal_visibility",
            "visibility_runtime",
            env["VISIBILITY_PASSWORD"],
            2,
        ),
    ]:
        stores[key] = {
            "sql": {
                "pluginName": "postgres12",
                "databaseName": database,
                "connectAddr": host + ":5432",
                "connectProtocol": "tcp",
                "user": user,
                "password": password,
                "maxConns": cap,
                "maxIdleConns": 1,
                "maxConnLifetime": "30m",
                "tls": {
                    "enabled": True,
                    "caFile": "/run/secrets/db-ca.pem",
                    "enableHostVerification": True,
                    "serverName": host,
                },
            }
        }
    tls = {
        "refreshInterval": "1m",
        "expirationChecks": {
            "warningWindow": "720h",
            "errorWindow": "168h",
            "checkInterval": "1h",
        },
    }
    for tier in ("frontend", "internode"):
        tls[tier] = {
            "server": {
                "requireClientAuth": True,
                "certFile": "/run/secrets/" + tier + ".crt",
                "keyFile": "/run/secrets/" + tier + ".key",
                "clientCaFiles": ["/run/secrets/ca.pem"],
            },
            "client": {
                "serverName": "temporal",
                "disableHostVerification": False,
                "rootCaFiles": ["/run/secrets/ca.pem"],
            },
        }
    services = {}
    for name, grpc, member in [
        ("frontend", 7233, 6933),
        ("history", 7234, 6934),
        ("matching", 7235, 6935),
        ("worker", 7239, 6939),
    ]:
        services[name] = {
            "rpc": {"grpcPort": grpc, "membershipPort": member, "bindOnIP": "0.0.0.0"}
        }
    return {
        "log": {"stdout": True, "level": "warn"},
        "persistence": {
            "numHistoryShards": 128,
            "defaultStore": "default",
            "visibilityStore": "visibility",
            "datastores": stores,
        },
        "global": {
            "membership": {
                "maxJoinDuration": "60s",
                "broadcastAddress": "__CONTAINER_IP__",
            },
            "tls": tls,
            "metrics": {
                "prometheus": {
                    "timerType": "histogram",
                    "listenAddress": "0.0.0.0:9090",
                }
            },
        },
        "services": services,
        "clusterMetadata": {
            "enableGlobalNamespace": False,
            "failoverVersionIncrement": 10,
            "masterClusterName": "primary",
            "currentClusterName": "primary",
            "clusterInformation": {
                "primary": {
                    "enabled": True,
                    "initialFailoverVersion": 1,
                    "rpcName": "frontend",
                    "rpcAddress": "temporal:7233",
                }
            },
        },
        "publicClient": {"hostPort": "temporal:7233"},
        "archival": {
            "history": {"state": "disabled", "enableRead": False},
            "visibility": {"state": "disabled", "enableRead": False},
        },
    }


def acl_config(env):
    lines = ["user default off"]
    # Separate users; administrative persistence operations are never available to applications.
    rules = {
        "api": "~agentclash:* ~run:* ~rate:* ~cooldown:* ~provider:* &run:* -@all +@read +@write +@connection +@pubsub +@scripting +multi +exec +discard +watch +unwatch +time -flushall -flushdb",
        "worker": "~agentclash:* ~run:* ~rate:* ~cooldown:* ~provider:* &run:* -@all +@read +@write +@connection +@pubsub +@scripting +multi +exec +discard +watch +unwatch +time -flushall -flushdb",
        "terminal": "~trycli:* -@all +ping +info +client +get +set +del +expire +incrbyfloat +eval",
        "admin": "~* &* +@all",
    }
    for user, rule in rules.items():
        password = env[user.upper() + "_PASSWORD"]
        require(len(password.encode()) >= 32, "Cache password is too short")
        lines.append(
            "user "
            + user
            + " on #"
            + hashlib.sha256(password.encode()).hexdigest()
            + " "
            + rule
        )
    return "\n".join(lines) + "\n"


def render(generation, payloads, *, set_owner=True):
    generation = Path(generation)
    uid = UIDS["temporal"] if set_owner else None
    write_private(
        generation / "temporal/production.template.json",
        json.dumps(temporal_config(payloads["temporal"]["env"])),
        uid,
    )
    uid = UIDS["valkey"] if set_owner else None
    write_private(
        generation / "valkey/users.acl", acl_config(payloads["valkey"]["env"]), uid
    )
