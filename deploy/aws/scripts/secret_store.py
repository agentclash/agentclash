#!/usr/bin/env python3
"""Load version-pinned per-service Secrets Manager JSON into a new tmpfs generation.

Secret shape: {"env": {"NAME": "exact value"}, "files": {"ca.pem": "PEM..."}}.
Only file names in the contract below are accepted; no archive extraction.
The calling rollout mounts the generation explicitly, never a mutable symlink.
"""

import json
import os
from pathlib import Path
import re
import shlex
import shutil
import tempfile
from urllib.parse import parse_qs, urlsplit

from common import account_guard, aws, require, write_private

# Independent mounts, even where upstream images use the same UID.
UIDS = {
    "api": 10001,
    "worker": 10001,
    "terminal": 10001,
    "app-schema": 10001,
    "temporal": 1000,
    "temporal-schema": 1000,
    "temporal-admin": 1000,
    "database-admin": 1000,
    "valkey": 999,
    "cache-admin": 999,
    "caddy": 10001,
}
FILES = {
    "ca.pem",
    "client.crt",
    "client.key",
    "server.crt",
    "server.key",
    "db-ca.pem",
    "cache-ca.pem",
    "internode.crt",
    "internode.key",
    "frontend.crt",
    "frontend.key",
}
RESERVED = {
    "PATH",
    "ENV",
    "BASH_ENV",
    "SHELLOPTS",
    "BASHOPTS",
    "IFS",
    "CDPATH",
    "NODE_OPTIONS",
    "NODE_TLS_REJECT_UNAUTHORIZED",
    "PYTHONPATH",
    "PYTHONHOME",
    "SSL_CERT_FILE",
    "SSL_CERT_DIR",
    "AWS_SHARED_CREDENTIALS_FILE",
    "AWS_CONFIG_FILE",
}


def validate(service, payload):
    require(service in UIDS, "Unknown secret recipient")
    require(
        isinstance(payload, dict) and set(payload) == {"env", "files"},
        "Invalid secret structure",
    )
    for group in ("env", "files"):
        require(isinstance(payload[group], dict), "Invalid secret map")
        for key, value in payload[group].items():
            require(
                isinstance(value, str) and "\x00" not in value and len(value) <= 65536,
                "Invalid secret value",
            )
            if group == "files":
                require(key in FILES, "Unapproved secret file name")
            else:
                require(
                    re.fullmatch(r"[A-Z][A-Z0-9_]*", key)
                    and key not in RESERVED
                    and not key.startswith(("LD_", "DYLD_", "AWS_")),
                    "Unapproved environment name",
                )
    env = payload["env"]
    if service in ("api", "worker"):
        require(
            env.get("APP_ENV") == "production", "Production application mode required"
        )
        require(
            env.get("TEMPORAL_TLS_ENABLED") == "true"
            and not env.get("TEMPORAL_API_KEY"),
            "Self-hosted mTLS required",
        )
        require(
            env.get("TEMPORAL_HOST_PORT") == "temporal:7233"
            and env.get("TEMPORAL_NAMESPACE") == "agentclash-prod"
            and env.get("TEMPORAL_TLS_SERVER_NAME") == "temporal",
            "Private Temporal destination required",
        )
        for key, file in [
            ("TEMPORAL_TLS_CA_FILE", "ca.pem"),
            ("TEMPORAL_TLS_CERT_FILE", "client.crt"),
            ("TEMPORAL_TLS_KEY_FILE", "client.key"),
        ]:
            require(
                env.get(key) == "/run/secrets/" + file and file in payload["files"],
                "Temporal TLS files required",
            )
    if service in ("api", "worker", "app-schema"):
        parsed = urlsplit(env.get("DATABASE_URL", ""))
        require(
            parsed.scheme in ("postgres", "postgresql")
            and parsed.hostname
            and parsed.password,
            "Database credentials required",
        )
        query = parse_qs(parsed.query)
        require(
            set(query)
            <= {
                "sslmode",
                "sslrootcert",
                "pool_max_conns",
                "pool_min_conns",
                "connect_timeout",
                "application_name",
            },
            "Unapproved database connection option",
        )
        require(
            parsed.username
            == ("app_owner" if service == "app-schema" else "app_runtime"),
            "Wrong database role",
        )
        require(
            query.get("sslmode") == ["verify-full"]
            and query.get("sslrootcert") == ["/run/secrets/db-ca.pem"]
            and "db-ca.pem" in payload["files"],
            "Verified database TLS required",
        )
        if service != "app-schema":
            require(
                query.get("pool_max_conns") == ["10"],
                "Application pool budget must be ten",
            )
    if service in ("api", "worker", "terminal"):
        cache = urlsplit(env.get("REDIS_URL", ""))
        require(
            cache.scheme == "rediss"
            and cache.hostname == "valkey"
            and cache.password
            and not cache.query,
            "Authenticated cache TLS required",
        )
        require(
            cache.username == service
            and cache.port == 6379
            and cache.path in ("", "/0")
            and not cache.fragment,
            "Wrong cache role or database",
        )
        require(
            env.get("REDIS_TLS_CA_FILE") == "/run/secrets/cache-ca.pem"
            and "cache-ca.pem" in payload["files"],
            "Cache CA required",
        )
    if service == "worker":
        require(
            env.get("WORKER_MAX_CONCURRENT_ACTIVITIES") == "2"
            and env.get("WORKER_MAX_CONCURRENT_WORKFLOW_TASKS") == "2"
            and env.get("SANDBOX_WARM_POOL_SIZE") == "0",
            "Explicit initial worker concurrency required",
        )
    if service == "terminal":
        require(
            env.get("NODE_ENV") == "production", "Production terminal mode required"
        )
        require(
            len(env.get("TRY_CLI_PROXY_SECRET", "").encode()) >= 32,
            "Terminal proxy secret required",
        )
        require(
            env.get("TRY_CLI_TRUSTED_PROXY_CIDRS") == "172.30.72.2/32",
            "Exact Caddy peer required",
        )
    if service == "temporal-schema":
        require(
            env.get("SQL_TLS") == "true"
            and env.get("SQL_TLS_CA_FILE") == "/run/secrets/db-ca.pem"
            and env.get("SQL_TLS_SERVER_NAME") == env.get("SQL_HOST")
            and env.get("SQL_HOST")
            and "db-ca.pem" in payload["files"]
            and env.get("SQL_PLUGIN") == "postgres12"
            and env.get("SQL_PORT") == "5432"
            and not env.get("SQL_TLS_DISABLE_HOST_VERIFICATION"),
            "Verified Temporal schema connection required",
        )
    if service == "database-admin":
        require(
            env.get("PGSSLMODE") == "verify-full"
            and env.get("PGSSLROOTCERT") == "/run/secrets/db-ca.pem"
            and env.get("PGHOST")
            and "db-ca.pem" in payload["files"],
            "Verified database admin TLS required",
        )
    if service == "temporal-admin":
        require(
            env.get("TEMPORAL_ADDRESS") == "temporal:7233"
            and env.get("TEMPORAL_TLS") == "true"
            and env.get("TEMPORAL_TLS_SERVER_NAME") == "temporal"
            and not env.get("TEMPORAL_TLS_DISABLE_HOST_VERIFICATION"),
            "Private Temporal admin mTLS required",
        )
        for key, file in [
            ("TEMPORAL_TLS_CA", "ca.pem"),
            ("TEMPORAL_TLS_CERT", "client.crt"),
            ("TEMPORAL_TLS_KEY", "client.key"),
        ]:
            require(
                env.get(key) == "/run/secrets/" + file and file in payload["files"],
                "Temporal admin TLS files required",
            )
    required_files = {
        "valkey": {"ca.pem", "server.crt", "server.key"},
        "temporal": {
            "ca.pem",
            "db-ca.pem",
            "frontend.crt",
            "frontend.key",
            "internode.crt",
            "internode.key",
        },
        "cache-admin": {"ca.pem"},
    }
    require(
        required_files.get(service, set()) <= set(payload["files"]),
        "Required service TLS files are missing",
    )
    if service == "cache-admin":
        require(
            len(env.get("REDISCLI_AUTH", "")) >= 32,
            "Cache admin authentication is required",
        )
    if service == "caddy":
        for key in ("API_DOMAIN", "TERMINAL_DOMAIN"):
            require(
                re.fullmatch(r"[a-z0-9]+(?:[.-][a-z0-9]+)+", env.get(key, "")),
                "Caddy requires DNS names only",
            )
        require(
            re.fullmatch(r"[^\s@{}]+@[^\s@{}]+\.[^\s@{}]+", env.get("ACME_EMAIL", "")),
            "Invalid ACME contact",
        )
    return payload


def materialize(parent, service, payload, *, set_owner=True):
    validate(service, payload)
    directory = Path(parent) / service
    directory.mkdir(mode=0o700)
    uid = UIDS[service] if set_owner else None
    if set_owner:
        os.chown(directory, uid, uid)
    script = "".join(
        "export " + key + "=" + shlex.quote(value) + "\n"
        for key, value in sorted(payload["env"].items())
    )
    write_private(directory / "env.sh", script, uid)
    for name, value in payload["files"].items():
        write_private(directory / name, value, uid)
    return directory


def load(settings, parent="/run/agentclash"):
    account_guard(settings)
    refs = settings["secrets"]
    require(
        set(refs) == set(UIDS), "Every service requires an explicit secret reference"
    )
    root = Path(parent)
    require(root.is_dir() and not root.is_symlink(), "Runtime tmpfs missing")
    mounts = Path("/proc/mounts").read_text().splitlines()
    require(
        any(row.split()[1:3] == [str(root), "tmpfs"] for row in mounts),
        "Secret delivery requires tmpfs",
    )
    payloads = {}
    for service, ref in refs.items():
        require(
            ref["arn"].startswith(
                "arn:aws:secretsmanager:"
                + settings["region"]
                + ":"
                + settings["account_id"]
                + ":secret:"
            ),
            "Secret belongs to a different account or region",
        )
        require(
            re.fullmatch(r"[A-Za-z0-9-]{32,64}", ref["version_id"]),
            "Secret version must be immutable",
        )
        data = json.loads(
            aws(
                settings,
                "secretsmanager",
                "get-secret-value",
                "--secret-id",
                ref["arn"],
                "--version-id",
                ref["version_id"],
                "--output",
                "json",
            )
        )
        payloads[service] = validate(service, json.loads(data["SecretString"]))
        env = payloads[service]["env"]
        database_host = (
            urlsplit(env["DATABASE_URL"]).hostname
            if "DATABASE_URL" in env
            else env.get("DB_HOST") or env.get("SQL_HOST") or env.get("PGHOST")
        )
        if database_host:
            require(
                database_host == settings["database_endpoint"],
                "Unapproved database endpoint",
            )
    generation = Path(tempfile.mkdtemp(prefix="generation-", dir=root))
    try:
        # Parent execute-only permission permits container UIDs to traverse their mounts.
        generation.chmod(0o711)
        for service, payload in payloads.items():
            materialize(generation, service, payload)
    except Exception:
        shutil.rmtree(generation)
        raise
    return generation, payloads
