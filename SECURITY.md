# Security Policy

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues,
discussions, or pull requests.**

Instead, use GitHub's private vulnerability reporting:

1. Go to the repository's **Security** tab.
2. Click **Report a vulnerability** to open a private advisory.

Include as much of the following as you can:

- A description of the issue and its impact.
- Steps to reproduce (proof-of-concept, affected component: backend API, Temporal
  worker, CLI, or web).
- Affected version(s) / commit, and any relevant configuration.

We aim to acknowledge a report within a few business days and will keep you
updated as we triage and fix. Please give us a reasonable window to ship a fix
before any public disclosure, and let us know if you'd like to be credited.

## Scope

AgentClash runs untrusted agent code in sandboxes and brokers provider/API
credentials, so we are especially interested in: sandbox escape, secret/credential
exposure, tenant isolation breaks, authentication/authorization bypass, and
injection in the API or workflow layer.

## Supported versions

This is an actively developed pre-1.0 project; security fixes target the latest
`main` and the most recent release. Please test against the latest version before
reporting.
