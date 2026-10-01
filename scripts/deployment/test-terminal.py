#!/usr/bin/env python3
"""Isolated terminal rehearsal. Requires Bun, Caddy, OpenSSL and Docker.

All certificates, runtime settings and command output stay outside the checkout.
Never accepts application credentials or a remote Redis endpoint.
"""
import argparse
import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[2]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--evidence-dir', type=Path, required=True)
    parser.add_argument('--test-path', default='tests')
    args = parser.parse_args()
    selected = (ROOT/'services/try-cli'/args.test_path).resolve()
    if not selected.is_relative_to(ROOT/'services/try-cli/tests'):
        raise SystemExit('Tests must be within services/try-cli/tests')
    evidence = args.evidence_dir.expanduser().resolve()
    if evidence == ROOT or ROOT in evidence.parents:
        raise SystemExit('Evidence must be outside the public checkout')
    os.umask(0o077)
    evidence.mkdir(parents=True, exist_ok=True, mode=0o700)
    run = Path(tempfile.mkdtemp(prefix='terminal-', dir=evidence))
    for executable in ['bun', 'caddy', 'openssl', 'docker', 'curl']:
        if not shutil.which(executable):
            raise SystemExit(f'Missing local dependency: {executable}')
    container = 'trycli-isolated-' + secrets.token_hex(6)
    env = {k: v for k, v in os.environ.items() if k in ['PATH', 'HOME', 'TMPDIR', 'DOCKER_HOST', 'DOCKER_CONTEXT']}
    log = (run / 'rehearsal.log').open('w')

    def call(argv, **kwargs):
        return subprocess.run(argv, env=env, stdout=log, stderr=subprocess.STDOUT, check=True, **kwargs)

    try:
        call(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
              '-subj', '/CN=Isolated terminal test CA', '-keyout', str(run/'ca.key'), '-out', str(run/'ca.pem')])
        call(['openssl', 'req', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=localhost',
              '-keyout', str(run/'server.key'), '-out', str(run/'server.csr')])
        (run/'server.ext').write_text('subjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n')
        call(['openssl', 'x509', '-req', '-days', '1', '-in', str(run/'server.csr'), '-CA', str(run/'ca.pem'),
              '-CAkey', str(run/'ca.key'), '-CAcreateserial', '-extfile', str(run/'server.ext'), '-out', str(run/'server.pem')])
        password = secrets.token_urlsafe(32)
        (run/'redis.conf').write_text('\n'.join([
            'port 0', 'tls-port 6379', 'bind 0.0.0.0', 'tls-auth-clients no',
            'tls-cert-file /test/server.pem', 'tls-key-file /test/server.key', 'tls-ca-cert-file /test/ca.pem',
            'requirepass ' + password, 'appendonly yes', 'appendfsync always', 'dir /data', 'save ""', '',
        ]))
        # Keep a fixed loopback binding across container restart (Docker may
        # reassign an automatically published ephemeral host port).
        with socket.socket() as reservation:
            reservation.bind(('127.0.0.1', 0))
            port = reservation.getsockname()[1]
        # Only this disposable container is ever started/restarted/removed.
        call(['docker', 'run', '-d', '--rm', '--name', container, '--user', '0:0', '--entrypoint', 'redis-server',
              '-p', f'127.0.0.1:{port}:6379', '-v', str(run) + ':/test:ro', 'redis:7-alpine', '/test/redis.conf'])
        binding = subprocess.check_output(['docker', 'port', container, '6379/tcp'], env=env, text=True).strip()
        port = int(binding.rsplit(':', 1)[1])
        for attempt in range(40):
            try:
                with socket.create_connection(('127.0.0.1', port), timeout=0.2):
                    break
            except OSError:
                time.sleep(0.1)
        env.update({
            'NODE_ENV': 'test', 'TRY_CLI_TEST_REDIS_URL': f'rediss://:{password}@localhost:{port}',
            'TRY_CLI_TEST_CA_FILE': str(run/'ca.pem'), 'TRY_CLI_TEST_PRIVATE_DIR': str(run),
            'TRY_CLI_TEST_CONTAINER': container,
        })
        call(['bun', 'install', '--frozen-lockfile'], cwd=ROOT/'services/try-cli')
        call(['bun', '--no-env-file', '--preserve-symlinks', 'test', args.test_path, '--timeout', '110000'], cwd=ROOT/'services/try-cli')
        call(['bun', 'run', 'typecheck'], cwd=ROOT/'services/try-cli')
        (run/'result.json').write_text(json.dumps({'status': 'passed', 'scope': 'isolated TLS Redis, Caddy, terminal HTTP/WS, synthetic sandboxes/providers'}, indent=2)+'\n')
        print('Terminal rehearsal passed; evidence saved privately.')
    finally:
        subprocess.run(['docker', 'logs', container], env=env, stdout=log, stderr=subprocess.STDOUT)
        subprocess.run(['docker', 'rm', '-fv', container], env=env, stdout=log, stderr=subprocess.STDOUT)
        for name in ['ca.key', 'server.key', 'redis.conf']:
            (run/name).unlink(missing_ok=True)
        log.close()


if __name__ == '__main__':
    main()
