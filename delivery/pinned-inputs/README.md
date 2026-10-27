# Retained public build inputs

These unmodified upstream packages keep the existing pinned build reproducible after mirror removal. The downloader always checks the original SHA256 before use. Upstream provenance and versions remain in `deploy/aws/platform/inputs.lock.json`. Package metadata and license declarations are retained inside each archive.

- v3.23/c-ares 1.34.8-r0 (`7a570d080d05e60b1256ea73586924bf9a25f468c57aae10d1cdfe7fcc3f93a2`); license = MIT.
- v3.24/ca-certificates 20260909-r0 (`d33f9058111586cea83cb3cca78d24efb36a45e6466fd0228f66e791f1cb79ed`); license = MPL-2.0 AND MIT.
- v3.24/ca-certificates-bundle 20260909-r0 (`a35f8cf2366d90294d0bb089af565fd17f3fa03721e61bcc7f02ed3d8334e7ea`); license = MPL-2.0 AND MIT.
- v3.24/tzdata 2026d-r0 (`e13f0f891c5e231bfb89727556bc05e9a44fce6389c6c1505f2772369da5a262`); license = Public-Domain.

## Notices and corresponding source

These are unmodified upstream packages; they are not governed by AgentClash's license. The original package metadata and signatures remain inside the archives. Upstream notices are retained in `notices/`.

- c-ares: [source and notices](https://github.com/c-ares/c-ares/tree/v1.34.8), [Alpine build source](https://github.com/alpinelinux/aports/tree/5d4c6f7c12416f51673a8364ae60a3046345f11f/main/c-ares).
- CA certificates and bundle: [corresponding source](https://gitlab.alpinelinux.org/alpine/ca-certificates/-/archive/20260909/ca-certificates-20260909.tar.bz2), [Alpine build source](https://github.com/alpinelinux/aports/tree/eb7078df3a16d666e8f7723f94bb92e00d4b9236/main/ca-certificates). Mozilla certificate source is governed by the [Mozilla Public License 2.0](https://www.mozilla.org/en-US/MPL/2.0/); package utilities retain their upstream notices.
- tzdata: [corresponding public-domain source](https://data.iana.org/time-zones/releases/tzdata2026d.tar.gz), [Alpine build source](https://github.com/alpinelinux/aports/tree/a19caf9fe771707618d4d9e4fa2dd9db8155b461/main/tzdata).
