#!/usr/bin/env python3
"""Offline private-CA issuance and expiry checks. No automatic publishing or rotation.

The CA directory belongs to the operator, outside the checkout and host runtime.
Existing CA keys are never replaced. Generated keys must be delivered through the
secret manager under the maintenance/trust-overlap procedure in the runbook.
"""

import argparse
import os
from pathlib import Path
import sys

from common import Refused, require, run

IDENTITIES = {
    "temporal": ("serverAuth,clientAuth", "DNS:temporal"),
    "valkey": ("serverAuth", "DNS:valkey"),
    "api": ("clientAuth", ""),
    "worker": ("clientAuth", ""),
    "operator": ("clientAuth", ""),
}


def outside_checkout(path):
    path = Path(path).resolve()
    root = Path(__file__).resolve().parents[3]
    require(
        not path.is_relative_to(root), "Private keys must remain outside the checkout"
    )
    return path


def ca_init(directory):
    directory = outside_checkout(directory)
    directory.mkdir(mode=0o700, parents=True, exist_ok=False)
    run(
        [
            "openssl",
            "req",
            "-x509",
            "-newkey",
            "rsa:3072",
            "-nodes",
            "-sha256",
            "-days",
            "1825",
            "-subj",
            "/CN=AgentClash private platform CA",
            "-addext",
            "basicConstraints=critical,CA:TRUE,pathlen:0",
            "-addext",
            "keyUsage=critical,keyCertSign,cRLSign",
            "-keyout",
            str(directory / "ca.key"),
            "-out",
            str(directory / "ca.pem"),
        ]
    )


def issue(ca, output, identity):
    require(identity in IDENTITIES, "Unknown certificate identity")
    ca, output = outside_checkout(ca), outside_checkout(output)
    require((ca / "ca.key").stat().st_mode & 0o077 == 0, "CA key must be private")
    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    usage, san = IDENTITIES[identity]
    ext = output / "extensions.cnf"
    ext.write_text(
        "basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage="
        + usage
        + "\n"
        + ("subjectAltName=" + san + "\n" if san else "")
    )
    run(
        [
            "openssl",
            "req",
            "-new",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-subj",
            "/CN=" + identity,
            "-keyout",
            str(output / "key.pem"),
            "-out",
            str(output / "request.csr"),
        ]
    )
    # Random serial avoids a shared mutable CA serial file and concurrent issuance races.
    import secrets

    serial = "0x" + secrets.token_hex(16)
    run(
        [
            "openssl",
            "x509",
            "-req",
            "-in",
            str(output / "request.csr"),
            "-CA",
            str(ca / "ca.pem"),
            "-CAkey",
            str(ca / "ca.key"),
            "-set_serial",
            serial,
            "-days",
            "90",
            "-sha256",
            "-extfile",
            str(ext),
            "-out",
            str(output / "cert.pem"),
        ]
    )
    run(["openssl", "verify", "-CAfile", str(ca / "ca.pem"), str(output / "cert.pem")])
    (output / "request.csr").unlink()
    ext.unlink()


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    init = sub.add_parser("init-ca")
    init.add_argument("directory")
    leaf = sub.add_parser("issue")
    leaf.add_argument("ca")
    leaf.add_argument("output")
    leaf.add_argument("identity", choices=IDENTITIES)
    check = sub.add_parser("check")
    check.add_argument("certificate")
    check.add_argument("--days", type=int, default=30)
    args = parser.parse_args()
    if args.command == "init-ca":
        ca_init(args.directory)
    elif args.command == "issue":
        issue(args.ca, args.output, args.identity)
    else:
        require(args.days >= 1, "Positive expiry window required")
        run(
            [
                "openssl",
                "x509",
                "-in",
                args.certificate,
                "-checkend",
                str(args.days * 86400),
                "-noout",
            ]
        )
    print("Certificate operation passed")


if __name__ == "__main__":
    try:
        main()
    except (Refused, OSError, ValueError):
        sys.exit("Certificate operation refused or failed; no material printed")
