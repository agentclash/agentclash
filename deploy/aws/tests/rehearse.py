#!/usr/bin/env python3
"""Isolated production-image rehearsal. All generated material stays outside Git.

Requires Docker (amd64 emulation on ARM), Go, Bun, OpenSSL, Python >=3.12.
No production credentials, provider calls, cloud mutations or source access.
"""

import argparse
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time

ROOT = Path(__file__).resolve().parents[3]
AWS = ROOT / "deploy/aws"
sys.path.insert(0, str(AWS / "scripts"))
sys.path.insert(0, str(ROOT / "delivery"))
from maintained import selection
from certificates import ca_init, issue, IDENTITIES
from database import initialize_sql, grants_sql, DATABASES
from render import temporal_config, acl_config
from secret_store import materialize
from restore import safe_extract


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument("--evidence-dir", required=True)
    parser.add_argument(
        "--images",
        required=True,
        help="Private image-selection.json from the scanned build",
    )
    parser.add_argument(
        "--cache-only", action="store_true", help="Run only cache integration checks"
    )
    args = parser.parse_args()
    evidence = Path(args.evidence_dir).resolve()
    if evidence.is_relative_to(ROOT):
        raise RuntimeError("Evidence must remain outside Git")
    evidence.mkdir(mode=0o700, parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="platform-", dir=evidence))
    work.chmod(0o700)
    images = selection(args.images)
    env = {
        k: v
        for k, v in os.environ.items()
        if k
        in (
            "PATH",
            "HOME",
            "USER",
            "TMPDIR",
            "GOPATH",
            "GOCACHE",
            "DOCKER_HOST",
            "DOCKER_CONTEXT",
        )
    }
    names = []
    net = "agentclash-platform-" + secrets.token_hex(5)
    step = 0

    def run(command, *, data=None, child_env=None, ok=True, timeout=300):
        nonlocal step
        step += 1
        result = subprocess.run(
            command,
            input=data,
            env=child_env or env,
            capture_output=True,
            timeout=timeout,
        )
        (work / (str(step) + ".log")).write_bytes(result.stdout + result.stderr)
        if ok and result.returncode:
            raise RuntimeError(
                "Rehearsal operation "
                + str(step)
                + " failed; private evidence retained"
            )
        return result

    def docker(*command, **kwargs):
        return run(["docker", *command], **kwargs)

    def ready(command, timeout=90):
        until = time.monotonic() + timeout
        while time.monotonic() < until:
            result = run(command, ok=False, timeout=15)
            if result.returncode == 0:
                return result.stdout
            time.sleep(0.5)
        raise RuntimeError("Readiness deadline exceeded")

    def private_dir(name):
        d = work / name
        d.mkdir(mode=0o700)
        return d

    def owner(path, uid):
        docker(
            "run",
            "--rm",
            "--user",
            "0:0",
            "-v",
            str(path) + ":/fixture",
            images["alpine"],
            "chown",
            "-R",
            str(uid) + ":" + str(uid),
            "/fixture",
        )

    def new_container(name, image, arguments, extra=()):
        full = net + "-" + name
        names.append(full)
        docker(
            "run",
            "-d",
            "--name",
            full,
            "--platform",
            "linux/amd64",
            "--network",
            net,
            *extra,
            image,
            *arguments,
        )
        return full

    def port(name, p):
        return (
            docker(
                "inspect",
                "--format",
                '{{(index (index .NetworkSettings.Ports "'
                + str(p)
                + '/tcp") 0).HostPort}}',
                name,
            )
            .stdout.decode()
            .strip()
        )

    def job(secret, image, arguments, *, data=None, ok=True):
        return docker(
            "run",
            "--rm",
            "--platform",
            "linux/amd64",
            "--network",
            net,
            "--user",
            "1000:1000",
            "-v",
            str(secret) + ":/run/secrets:ro",
            "-v",
            str(AWS / "config/secret-exec.sh") + ":/opt/secret-exec.sh:ro",
            "-v",
            str(AWS / "config/temporal-schema.sh") + ":/opt/temporal-schema.sh:ro",
            "--entrypoint",
            "/bin/sh",
            image,
            "/opt/secret-exec.sh",
            *arguments,
            data=data,
            ok=ok,
        )

    try:
        print(
            "Preparing isolated CA, per-service secrets and production images",
            flush=True,
        )
        docker("network", "create", net)
        ca_init(work / "ca")
        ca_init(work / "untrusted")
        IDENTITIES["postgres"] = ("serverAuth", "DNS:postgres")
        for identity in ("postgres", "temporal", "valkey", "operator"):
            issue(work / "ca", work / ("cert-" + identity), identity)
        if not args.cache_only:
            pgsecret = private_dir("postgres-secret")
            shutil.copy(work / "cert-postgres/cert.pem", pgsecret / "server.crt")
            shutil.copy(work / "cert-postgres/key.pem", pgsecret / "server.key")
            (pgsecret / "password").write_text(secrets.token_hex(24))
            owner(pgsecret, 70)
            pg = new_container(
                "postgres",
                images["postgres"],
                [
                    "postgres",
                    "-c",
                    "ssl=on",
                    "-c",
                    "ssl_cert_file=/run/pg/server.crt",
                    "-c",
                    "ssl_key_file=/run/pg/server.key",
                    "-c",
                    "max_connections=160",
                    "-c",
                    "log_min_error_statement=panic",
                ],
                [
                    "--network-alias",
                    "postgres",
                    "-v",
                    str(pgsecret) + ":/run/pg:ro",
                    "--tmpfs",
                    "/var/lib/postgresql:rw",
                    "-e",
                    "POSTGRES_PASSWORD_FILE=/run/pg/password",
                    "-e",
                    "POSTGRES_USER=platform_admin",
                ],
            )
            # The image's initialization server accepts Unix-socket probes, then
            # exits. TCP readiness waits for the final server before role setup.
            ready(
                [
                    "docker",
                    "exec",
                    pg,
                    "pg_isready",
                    "-h",
                    "127.0.0.1",
                    "-U",
                    "platform_admin",
                ]
            )

            def psql(sql, db="postgres", ok=True):
                return docker(
                    "exec",
                    "-i",
                    pg,
                    "psql",
                    "-X",
                    "-U",
                    "platform_admin",
                    "-d",
                    db,
                    "-v",
                    "ON_ERROR_STOP=1",
                    "-At",
                    data=sql.encode(),
                    ok=ok,
                )

            passwords = {
                role: secrets.token_hex(24)
                for pair in DATABASES.values()
                for role in pair
            }
            psql(initialize_sql(passwords))
            for db in DATABASES:
                psql(grants_sql(db))
            schema = private_dir("schema-parent")
            schema_dir = materialize(
                schema,
                "temporal-schema",
                {
                    "env": {
                        "SQL_HOST": "postgres",
                        "SQL_PORT": "5432",
                        "SQL_PLUGIN": "postgres12",
                        "SQL_TLS": "true",
                        "SQL_TLS_CA_FILE": "/run/secrets/db-ca.pem",
                        "SQL_TLS_SERVER_NAME": "postgres",
                        "TEMPORAL_OWNER_PASSWORD": passwords["temporal_owner"],
                        "VISIBILITY_OWNER_PASSWORD": passwords["visibility_owner"],
                    },
                    "files": {"db-ca.pem": (work / "ca/ca.pem").read_text()},
                },
                set_owner=False,
            )
            owner(schema_dir, 1000)
            for db in ("temporal", "temporal_visibility"):
                job(
                    schema_dir,
                    images["temporal-admin"],
                    ["/bin/sh", "/opt/temporal-schema.sh", db, "init"],
                )
                job(
                    schema_dir,
                    images["temporal-admin"],
                    ["/bin/sh", "/opt/temporal-schema.sh", db, "upgrade"],
                )
                psql(grants_sql(db))
            app_parent = private_dir("app-schema-parent")
            app_secret = materialize(
                app_parent,
                "app-schema",
                {
                    "env": {
                        "DATABASE_URL": "postgres://app_owner:"
                        + passwords["app_owner"]
                        + "@postgres:5432/agentclash?sslmode=verify-full&sslrootcert=/run/secrets/db-ca.pem",
                        "MIGRATION_DIR": "/migrations",
                    },
                    "files": {"db-ca.pem": (work / "ca/ca.pem").read_text()},
                },
                set_owner=False,
            )
            owner(app_secret, 1000)
            for _ in range(2):
                job(app_secret, images["app-schema"], ["/migrator"])
            psql(grants_sql("agentclash"))
            # Permissions fail under SET ROLE even though the test administrator is a superuser.
            for db, (_, runtime) in DATABASES.items():
                if (
                    psql(
                        "SET ROLE "
                        + runtime
                        + "; CREATE TABLE must_not_exist(id int);",
                        db,
                        ok=False,
                    ).returncode
                    == 0
                ):
                    raise RuntimeError("Runtime role unexpectedly has DDL")
            psql(
                "CREATE TABLE preserved(id int PRIMARY KEY, payload text); INSERT INTO preserved VALUES (1,'fixture');",
                "agentclash",
            )
            temporal_secret = private_dir("temporal-secret")

            def temporal_files(ca_directory, cert_directory):
                for dest, source in [
                    ("ca.pem", ca_directory / "ca.pem"),
                    ("db-ca.pem", work / "ca/ca.pem"),
                    ("frontend.crt", cert_directory / "cert.pem"),
                    ("frontend.key", cert_directory / "key.pem"),
                    ("internode.crt", cert_directory / "cert.pem"),
                    ("internode.key", cert_directory / "key.pem"),
                ]:
                    shutil.copy(source, temporal_secret / dest)
                (temporal_secret / "production.template.json").write_text(
                    json.dumps(
                        temporal_config(
                            {
                                "DB_HOST": "postgres",
                                "DB_PASSWORD": passwords["temporal_runtime"],
                                "VISIBILITY_PASSWORD": passwords["visibility_runtime"],
                            }
                        )
                    )
                )
                owner(temporal_secret, 1000)

            temporal_files(work / "ca", work / "cert-temporal")

            def start_temporal():
                return new_container(
                    "temporal",
                    images["temporal"],
                    [],
                    [
                        "--network-alias",
                        "temporal",
                        "--user",
                        "1000:1000",
                        "-p",
                        "127.0.0.1::7233",
                        "--memory",
                        "2560m",
                        "--read-only",
                        "--tmpfs",
                        "/tmp:rw,mode=1777",
                        "-v",
                        str(temporal_secret) + ":/run/secrets:ro",
                        "-v",
                        str(AWS / "config/temporal-entrypoint.sh") + ":/entry.sh:ro",
                        "--entrypoint",
                        "/entry.sh",
                    ],
                )

            # Execute via /bin/sh so the source file does not need an executable bit.
            entry = AWS / "config/temporal-entrypoint.sh"
            entry.chmod(0o755)
            temporal = start_temporal()
            client_secret = private_dir("client-parent")

            def client_files(ca_directory, cert_directory):
                d = client_secret / "temporal-admin"
                if d.exists():
                    owner(d, os.getuid())
                    shutil.rmtree(d)
                d = materialize(
                    client_secret,
                    "temporal-admin",
                    {
                        "env": {
                            "TEMPORAL_ADDRESS": "temporal:7233",
                            "TEMPORAL_NAMESPACE": "agentclash-prod",
                            "TEMPORAL_TLS": "true",
                            "TEMPORAL_TLS_SERVER_NAME": "temporal",
                            "TEMPORAL_TLS_CA": "/run/secrets/ca.pem",
                            "TEMPORAL_TLS_CERT": "/run/secrets/client.crt",
                            "TEMPORAL_TLS_KEY": "/run/secrets/client.key",
                        },
                        "files": {
                            "ca.pem": (ca_directory / "ca.pem").read_text(),
                            "client.crt": (cert_directory / "cert.pem").read_text(),
                            "client.key": (cert_directory / "key.pem").read_text(),
                        },
                    },
                    set_owner=False,
                )
                owner(d, 1000)
                return d

            cli = client_files(work / "ca", work / "cert-operator")

            def temporal_ready():
                until = time.monotonic() + 120
                while time.monotonic() < until:
                    if (
                        job(
                            cli,
                            images["temporal-admin"],
                            [
                                "temporal",
                                "operator",
                                "cluster",
                                "health",
                                "--command-timeout",
                                "5s",
                            ],
                            ok=False,
                        ).returncode
                        == 0
                    ):
                        return
                    time.sleep(1)
                docker("logs", temporal, ok=False)
                raise RuntimeError("Production Temporal did not become ready")

            temporal_ready()
            job(
                cli,
                images["temporal-admin"],
                [
                    "temporal",
                    "operator",
                    "namespace",
                    "create",
                    "--namespace",
                    "agentclash-prod",
                    "--retention",
                    "30d",
                ],
            )
            bad = job(
                cli,
                images["temporal-admin"],
                [
                    "temporal",
                    "operator",
                    "cluster",
                    "health",
                    "--tls-cert",
                    "",
                    "--tls-key",
                    "",
                    "--command-timeout",
                    "3s",
                ],
                ok=False,
            )
            if bad.returncode == 0:
                raise RuntimeError("Anonymous Temporal client accepted")
            print(
                "Temporal schemas, verified SQL TLS and private mTLS server passed",
                flush=True,
            )

            def sdk(phase, ca_directory, cert_directory):
                e = dict(
                    env,
                    PLATFORM_TEMPORAL_ADDRESS="127.0.0.1:" + port(temporal, 7233),
                    PLATFORM_CA=str(ca_directory / "ca.pem"),
                    PLATFORM_CERT=str(cert_directory / "cert.pem"),
                    PLATFORM_KEY=str(cert_directory / "key.pem"),
                    PLATFORM_UNTRUSTED_CA=str(work / "untrusted/ca.pem"),
                    PLATFORM_PHASE=phase,
                )
                return run(
                    [
                        "go",
                        "test",
                        "-race",
                        "-count=1",
                        "-tags=awsplatform",
                        "-timeout=150s",
                        "-run",
                        "^TestAWSPlatformTemporal$",
                        "./internal/temporalutil",
                    ],
                    child_env=e,
                    timeout=180,
                )

            # Go module working directory is explicit without altering source configuration.
            os.chdir(ROOT / "backend")
            sdk("seed", work / "ca", work / "cert-operator")
            # The delivery driver consumes the pinned CLI's legacy poller JSON.
            # Verify that wire shape against real SDK pollers, not only fixtures.
            sys.path.insert(0, str(ROOT / "delivery"))
            from health import poller_times

            for queue in ("execution", "scoring", "background"):
                for kind in ("workflow", "activity"):
                    result = job(
                        cli,
                        images["temporal-admin"],
                        [
                            "temporal",
                            "task-queue",
                            "describe",
                            "--task-queue",
                            queue,
                            "--legacy-mode",
                            "--task-queue-type-legacy",
                            kind,
                            "--output",
                            "json",
                        ],
                    )
                    if not poller_times(json.loads(result.stdout)):
                        raise RuntimeError(
                            "Pinned CLI poller response did not match the delivery check"
                        )
            old_ip = docker(
                "inspect",
                "--format",
                "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
                temporal,
            ).stdout.strip()
            docker("stop", "--time", "120", temporal)
            docker("rm", temporal)
            new_container(
                "ip-reservation",
                images["alpine"],
                ["sleep", "600"],
                ["--ip", old_ip.decode()],
            )
            ca_init(work / "ca-rotated")
            issue(work / "ca-rotated", work / "cert-temporal-rotated", "temporal")
            issue(work / "ca-rotated", work / "cert-operator-rotated", "operator")
            owner(temporal_secret, os.getuid())
            temporal_files(work / "ca-rotated", work / "cert-temporal-rotated")
            cli = client_files(work / "ca-rotated", work / "cert-operator-rotated")
            temporal = start_temporal()
            temporal_ready()
            new_ip = docker(
                "inspect",
                "--format",
                "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
                temporal,
            ).stdout.strip()
            if new_ip == old_ip:
                raise RuntimeError("Membership IP did not change")
            sdk("recover", work / "ca-rotated", work / "cert-operator-rotated")
            print(
                "Three queues recovered durable workflows after IP replacement and CA rotation",
                flush=True,
            )
            # Full isolated logical backup / restore into the same now-empty DB names.
            docker("stop", "--time", "120", temporal)
            docker("rm", temporal)
            for db, (owner_role, _) in DATABASES.items():
                dump = docker(
                    "exec",
                    pg,
                    "pg_dump",
                    "-U",
                    "platform_admin",
                    "-Fc",
                    "--no-owner",
                    "--no-acl",
                    "-d",
                    db,
                ).stdout
                (work / (db + ".dump")).write_bytes(dump)
                psql(
                    "DROP DATABASE "
                    + db
                    + " WITH (FORCE); CREATE DATABASE "
                    + db
                    + " OWNER "
                    + owner_role
                    + ";"
                )
                docker(
                    "exec",
                    "-i",
                    pg,
                    "pg_restore",
                    "-U",
                    "platform_admin",
                    "--no-owner",
                    "--no-acl",
                    "--role",
                    owner_role,
                    "--exit-on-error",
                    "-d",
                    db,
                    data=dump,
                )
                psql(grants_sql(db))
            if (
                psql(
                    "SELECT payload FROM preserved WHERE id=1;", "agentclash"
                ).stdout.strip()
                != b"fixture"
            ):
                raise RuntimeError("Application backup mismatch")
            temporal = start_temporal()
            temporal_ready()
            sdk("restore", work / "ca-rotated", work / "cert-operator-rotated")
            print(
                "Three-database backup/restore and Temporal visibility passed",
                flush=True,
            )
        os.chdir(ROOT / "backend")
        # Valkey with the production TLS/ACL/AOF configuration.
        cache_secret = private_dir("cache-secret")
        cache_data = private_dir("cache-data")
        for dst, src in [
            ("ca.pem", work / "ca/ca.pem"),
            ("server.crt", work / "cert-valkey/cert.pem"),
            ("server.key", work / "cert-valkey/key.pem"),
        ]:
            shutil.copy(src, cache_secret / dst)
        cache_passwords = {
            u + "_PASSWORD": secrets.token_hex(24)
            for u in ("API", "WORKER", "TERMINAL", "ADMIN")
        }
        (cache_secret / "users.acl").write_text(acl_config(cache_passwords))
        owner(cache_secret, 999)
        owner(cache_data, 999)
        import socket

        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            cache_port = listener.getsockname()[1]
        cache = new_container(
            "valkey",
            images["valkey"],
            ["/etc/valkey.conf"],
            [
                "--network-alias",
                "valkey",
                "--user",
                "999:999",
                "--entrypoint",
                "valkey-server",
                "-v",
                str(cache_secret) + ":/run/secrets:ro",
                "-v",
                str(cache_data) + ":/data",
                "-v",
                str(AWS / "config/valkey.conf") + ":/etc/valkey.conf:ro",
                "-p",
                "127.0.0.1:" + str(cache_port) + ":6379",
            ],
        )
        ce = dict(env, REDISCLI_AUTH=cache_passwords["ADMIN_PASSWORD"])

        def valkey(*command, ok=True):
            return docker(
                "exec",
                "-e",
                "REDISCLI_AUTH",
                cache,
                "valkey-cli",
                "--tls",
                "--cacert",
                "/run/secrets/ca.pem",
                "--user",
                "admin",
                "-h",
                "valkey",
                "--raw",
                *command,
                child_env=ce,
                ok=ok,
            )

        for _ in range(60):
            if valkey("PING", ok=False).stdout.strip() == b"PONG":
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("Valkey readiness failed")
        valkey("SET", "trycli:trial:used:fixture", "1", "EX", "3600")
        valkey("SET", "trycli:gw:daily:fixture", "1.25", "EX", "7200")
        expiry = valkey("PEXPIRETIME", "trycli:trial:used:fixture").stdout.strip()
        valkey("BGREWRITEAOF")
        for _ in range(120):
            if b"aof_rewrite_in_progress:0" in valkey("INFO", "persistence").stdout:
                break
            time.sleep(0.1)
        docker("restart", cache)
        for _ in range(60):
            if valkey("PING", ok=False).stdout.strip() == b"PONG":
                break
            time.sleep(0.5)
        if (
            valkey("GET", "trycli:gw:daily:fixture").stdout.strip() != b"1.25"
            or valkey("PEXPIRETIME", "trycli:trial:used:fixture").stdout.strip()
            != expiry
        ):
            raise RuntimeError("Durable limits changed")
        pe = dict(
            env,
            PLATFORM_REDIS_URL="rediss://worker:"
            + cache_passwords["WORKER_PASSWORD"]
            + "@127.0.0.1:"
            + port(cache, 6379),
            PLATFORM_REDIS_CA=str(work / "ca/ca.pem"),
            PLATFORM_UNTRUSTED_CA=str(work / "untrusted/ca.pem"),
        )
        # The server certificate covers valkey, so connect through a verified local TCP tunnel name via URL serverName is not supported.
        # Use a separate localhost test certificate with identical CA/usage for the host client only.
        IDENTITIES["valkey-local"] = (
            "serverAuth",
            "DNS:valkey,DNS:localhost,IP:127.0.0.1",
        )
        issue(work / "ca", work / "cert-valkey-local", "valkey-local")
        owner(cache_secret, os.getuid())
        shutil.copy(work / "cert-valkey-local/cert.pem", cache_secret / "server.crt")
        shutil.copy(work / "cert-valkey-local/key.pem", cache_secret / "server.key")
        owner(cache_secret, 999)
        docker("restart", cache)
        time.sleep(1)
        run(
            [
                "go",
                "test",
                "-race",
                "-count=1",
                "-tags=awsplatform",
                "-timeout=30s",
                "-run",
                "^TestAWSValkeyTransport$",
                "./internal/pubsub",
            ],
            child_env=pe,
        )
        # Exercise the actual Bun limit client and its narrow ACL, without providers.
        probe = work / "limits-probe.ts"
        probe.write_text(
            "import {createLimits} from "
            + json.dumps(str(ROOT / "services/try-cli/server/limits.ts"))
            + ";\n"
            + "const limits=createLimits({production:true,redisUrl:process.env.TEST_REDIS_URL,redisCaFile:process.env.TEST_CA,trialSeconds:3600} as any);\n"
            + 'await limits.health(); if(!await limits.claimTrial("fixture-bun")) throw Error("trial"); const h=await limits.reserve(.05,5); if(!h) throw Error("hold"); await h.settle(.01); limits.close();\n'
        )
        # Run the actual Linux/Bun application image, with secrets exported after
        # container creation. No test credential enters Docker's configured env.
        probe_parent = private_dir("bun-probe-parent")
        probe_secret = materialize(
            probe_parent,
            "cache-admin",
            {
                "env": {
                    "REDISCLI_AUTH": cache_passwords["ADMIN_PASSWORD"],
                    "TEST_REDIS_URL": "rediss://terminal:"
                    + cache_passwords["TERMINAL_PASSWORD"]
                    + "@valkey:6379",
                    "TEST_CA": "/run/secrets/ca.pem",
                },
                "files": {"ca.pem": (work / "ca/ca.pem").read_text()},
            },
            set_owner=False,
        )
        owner(probe_secret, 1000)
        probe_source = probe.read_text().replace(
            str(ROOT / "services/try-cli/server/limits.ts"),
            "/app/services/try-cli/server/limits.ts",
        )
        job(
            probe_secret,
            images["terminal"],
            [
                "bun",
                "--no-install",
                "--no-env-file",
                "--preserve-symlinks",
                "--eval",
                probe_source,
            ],
        )
        # Offline exact AOF backup, restore and fail-closed truncation proof.
        docker("stop", cache)
        owner(cache_data, os.getuid())
        backup = work / "cache.tar"
        with tarfile.open(backup, "w") as archive:
            for name in ("dump.rdb", "appendonlydir"):
                archive.add(cache_data / name, arcname=name)
        shutil.rmtree(cache_data)
        cache_data.mkdir(mode=0o700)
        safe_extract(backup, cache_data)
        owner(cache_data, 999)
        docker("start", cache)
        time.sleep(1)
        if valkey("GET", "trycli:gw:daily:fixture").stdout.strip() != b"1.25":
            raise RuntimeError("Cache restore mismatch")
        docker("stop", cache)
        owner(cache_data, os.getuid())
        tail = next((cache_data / "appendonlydir").glob("*.incr.aof"))
        with tail.open("ab") as stream:
            stream.write(b"*3\r\n$3\r\nSET\r\n$8\r\npartial")
        owner(cache_data, 999)
        docker("start", cache)
        time.sleep(2)
        code = docker(
            "inspect", "--format", "{{.State.Status}}/{{.State.ExitCode}}", cache
        ).stdout.strip()
        if not code.startswith(b"exited/") or code == b"exited/0":
            raise RuntimeError("Truncated AOF accepted")
        print(
            "Valkey TLS, Go/Bun clients, ACLs, Lua/pubsub, AOF restart/restore and corrupt-tail rejection passed",
            flush=True,
        )
        (work / "result.json").write_text(
            json.dumps(
                {
                    "status": "passed",
                    "images": images,
                    "checks": [
                        "three databases and distinct non-DDL runtime roles",
                        "Temporal SQL schema init and repeat upgrade",
                        "SQL TLS and Temporal mTLS",
                        "three queues durable timer recovery after membership IP change",
                        "CA/leaf rotation",
                        "three-database backup restore and visibility",
                        "Go and Bun Valkey TLS with ACLs",
                        "Lua, transactions and pubsub",
                        "durable absolute TTL through restart/AOF rewrite and restore",
                        "truncated AOF rejected",
                    ],
                },
                indent=2,
            )
            + "\n"
        )
        if args.cache_only:
            result_path = work / "result.json"
            result = json.loads(result_path.read_text())
            result["checks"] = result["checks"][-4:]
            result["scope"] = "cache-only"
            result_path.write_text(json.dumps(result, indent=2) + "\n")
    finally:
        # Remove only this run's resources and all generated keys/data. Logs remain private.
        for name in reversed(list(dict.fromkeys(names))):
            docker("inspect", name, ok=False)
            docker("logs", name, ok=False)
            docker("rm", "-f", name, ok=False)
        docker("network", "rm", net, ok=False)
        for directory in list(work.iterdir()):
            if directory.is_dir():
                owner(directory, os.getuid())
                shutil.rmtree(directory)
        for file in work.glob("*.dump"):
            file.unlink()
        for file in work.glob("*.tar"):
            file.unlink()
        for file in work.glob("*.ts"):
            file.unlink()


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        sys.exit(
            str(error)
            if isinstance(error, RuntimeError)
            else "Isolated rehearsal failed; inspect private evidence"
        )
