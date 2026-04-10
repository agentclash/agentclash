#!/usr/bin/env python3
import ipaddress
import json
import os
import socket
import sys
from urllib.parse import urlparse

import httpx


def fail(message: str, exit_code: int = 1) -> None:
    print(message, file=sys.stderr)
    raise SystemExit(exit_code)


def load_request(path: str) -> dict:
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)


def is_blocked_ip(ip: str, allowlist: list[str]) -> bool:
    address = ipaddress.ip_address(ip)
    for cidr in allowlist:
        if address in ipaddress.ip_network(cidr, strict=False):
            return False
    return (
        address.is_private
        or address.is_loopback
        or address.is_link_local
        or address.is_reserved
        or address.is_multicast
    )


def validate_target(raw_url: str, allowlist: list[str]) -> None:
    parsed = urlparse(raw_url)
    if parsed.scheme not in {"http", "https"}:
        fail("unsupported URL scheme")
    if not parsed.hostname:
        fail("url host is required")

    host = parsed.hostname
    try:
        ip = ipaddress.ip_address(host)
        if is_blocked_ip(str(ip), allowlist):
            fail("target host is blocked by network policy")
        return
    except ValueError:
        pass

    try:
        infos = socket.getaddrinfo(host, parsed.port or (443 if parsed.scheme == "https" else 80), type=socket.SOCK_STREAM)
    except socket.gaierror as exc:
        fail(f"dns resolution failed: {exc}")

    for info in infos:
        resolved_ip = info[4][0]
        if is_blocked_ip(resolved_ip, allowlist):
            fail("target host resolves to a blocked address")


def bounded_content(response: httpx.Response, limit: int) -> bytes:
    collected = bytearray()
    for chunk in response.iter_bytes():
        collected.extend(chunk)
        if len(collected) > limit:
            fail("response body exceeds size limit")
    return bytes(collected)


def main() -> None:
    if len(sys.argv) != 2:
        fail("usage: http_request.py <request.json>")

    # The request dict carries resolved ${secrets.*} values (typically
    # in Authorization headers) when a composed tool substituted them
    # at registry-build time. We MUST NOT let any exception handler
    # dump those back to stderr: stderr flows through to the tool
    # result error message path, which would leak the secret to the
    # agent. Every uncaught failure is reformatted to a type-only
    # sanitized string before calling fail(). The wrapper covers the
    # FULL main-body (including URL validation and body size checks),
    # not just the HTTP exchange — an exception in urlparse or
    # socket.getaddrinfo could otherwise stringify the URL with
    # userinfo attached and leak. See issue #186.
    try:
        request = load_request(sys.argv[1])
        allowlist = request.get("network_allowlist") or []
        validate_target(request["url"], allowlist)

        body = request.get("body") or ""
        max_request_body_bytes = int(request.get("max_request_body_bytes") or 0)
        if max_request_body_bytes > 0 and len(body.encode("utf-8")) > max_request_body_bytes:
            fail("request body exceeds size limit")

        timeout_seconds = int(request.get("timeout_seconds") or 30)
        output_path = (request.get("output_path") or "").strip()
        max_response_body_bytes = int(request.get("max_response_body_bytes") or 0)

        with httpx.Client(timeout=timeout_seconds, follow_redirects=True) as client:
            response = client.request(
                request["method"],
                request["url"],
                headers=request.get("headers") or {},
                content=body.encode("utf-8") if body else None,
            )

            payload = {
                "status_code": response.status_code,
                "headers": dict(response.headers),
                "url": str(response.url),
            }

            if output_path:
                content = bounded_content(response, max_response_body_bytes)
                os.makedirs(os.path.dirname(output_path), exist_ok=True)
                with open(output_path, "wb") as fh:
                    fh.write(content)
                payload["output_path"] = output_path
                payload["bytes_downloaded"] = len(content)
            else:
                content = bounded_content(response, max_response_body_bytes)
                payload["body"] = content.decode(response.encoding or "utf-8", errors="replace")
                payload["body_bytes"] = len(content)
    except SystemExit:
        # fail() raises SystemExit with a sanitized message already
        # printed — re-raise so the exit code propagates.
        raise
    except httpx.TimeoutException:
        fail("http request timed out")
    except httpx.ConnectError:
        fail("http connection error")
    except httpx.HTTPError as exc:
        # Only the exception class name — not str(exc), which may
        # include URL or headers depending on the httpx release.
        fail(f"http error: {type(exc).__name__}")
    except Exception as exc:
        fail(f"unexpected http_request failure: {type(exc).__name__}")

    print(json.dumps(payload))


if __name__ == "__main__":
    main()
