# Retained public build inputs

These unmodified upstream packages keep the existing pinned build reproducible after mirror removal. The downloader always checks the original SHA256 before use. Upstream provenance and versions remain in `deploy/aws/platform/inputs.lock.json` and `deploy/aws/platform/pdf-runtime.lock.json`. Package metadata and license declarations are retained inside each archive.

- v3.23/c-ares 1.34.8-r0 (`7a570d080d05e60b1256ea73586924bf9a25f468c57aae10d1cdfe7fcc3f93a2`); license = MIT.
- v3.24/ca-certificates 20260909-r0 (`d33f9058111586cea83cb3cca78d24efb36a45e6466fd0228f66e791f1cb79ed`); license = MPL-2.0 AND MIT.
- v3.24/ca-certificates-bundle 20260909-r0 (`a35f8cf2366d90294d0bb089af565fd17f3fa03721e61bcc7f02ed3d8334e7ea`); license = MPL-2.0 AND MIT.
- v3.24/tzdata 2026d-r0 (`e13f0f891c5e231bfb89727556bc05e9a44fce6389c6c1505f2772369da5a262`); license = Public-Domain.
- v3.22/zlib 1.3.2-r1 (`af24e502645e2b392cae833856d776691ac21655362e5db69d7b89f147f76216`); license = Zlib.
- v3.23/zlib 1.3.2-r1 (`ee62e1864c235901e2aa4d1868c7ae8f7cc5d0740a98c48a2525332eb4bfda73`); license = Zlib.
- v3.24/zlib 1.3.2-r1 (`63aeea03c15a2f9018f81805cfc8aa926bdf5cd68921f22149c2fbb5d0ee9f47`); license = Zlib.
- v3.24/libcrypto3 3.5.9-r0 (`6632d758d8f5e9ea3b650fe966f23bbf9a202f8b8dceecac93da135dec5e3689`); license = Apache-2.0.
- v3.24/libssl3 3.5.9-r0 (`05e3393fb95aa5751ca2f9d242f659f6cff82c1cc7767cc2df4a086f7ad01877`); license = Apache-2.0.
- v3.24/py3-pip 26.1.2-r0 (`8ab13441fd0abfd625ac4e91eed5648d90a6422359d45469307bd78ad4f1b223`); license = MIT.
- v3.24/py3-pip-pyc 26.1.2-r0 (`b7a66a9b68224468f9398220df4620f74747ee98e7526c2968c469c09a04cc37`); license = MIT.

Alpine removed the three zlib packages from its mirrors after zlib 1.4-r0. Alpine replaced the OpenSSL and pip packages with newer releases, so the mirrors can remove them next. A change to a newer package is a platform release, and it needs the platform update procedure in `delivery/PLATFORM-UPDATE.md`.

## Notices and corresponding source

These are unmodified upstream packages; they are not governed by AgentClash's license. The original package metadata and signatures remain inside the archives. Upstream notices are retained in `notices/`.

- c-ares: [source and notices](https://github.com/c-ares/c-ares/tree/v1.34.8), [Alpine build source](https://github.com/alpinelinux/aports/tree/5d4c6f7c12416f51673a8364ae60a3046345f11f/main/c-ares).
- CA certificates and bundle: [corresponding source](https://gitlab.alpinelinux.org/alpine/ca-certificates/-/archive/20260909/ca-certificates-20260909.tar.bz2), [Alpine build source](https://github.com/alpinelinux/aports/tree/eb7078df3a16d666e8f7723f94bb92e00d4b9236/main/ca-certificates). Mozilla certificate source is governed by the [Mozilla Public License 2.0](https://www.mozilla.org/en-US/MPL/2.0/); package utilities retain their upstream notices.
- tzdata: [corresponding public-domain source](https://data.iana.org/time-zones/releases/tzdata2026d.tar.gz), [Alpine build source](https://github.com/alpinelinux/aports/tree/a19caf9fe771707618d4d9e4fa2dd9db8155b461/main/tzdata).
- zlib: [corresponding source](https://github.com/madler/zlib/tree/v1.3.2), Alpine build source for [v3.22](https://github.com/alpinelinux/aports/tree/121fc97db0e7c59db704891b6d7ef83ae5df2cc2/main/zlib), [v3.23](https://github.com/alpinelinux/aports/tree/b92621a82657b055485976b304462bddf9aa9183/main/zlib) and [v3.24](https://github.com/alpinelinux/aports/tree/0afa2da0e8c8051c6f8f64a7a388e5a259904245/main/zlib). The license is in `notices/zlib-LICENSE.txt`.
- OpenSSL (libcrypto3 and libssl3): [corresponding source](https://github.com/openssl/openssl/tree/openssl-3.5.9), [Alpine build source](https://github.com/alpinelinux/aports/tree/29b9ec24b1b5b39aeef51fa2a044210e2ec5258e/main/openssl). The license is in `notices/openssl-LICENSE.txt`.
- pip (py3-pip and py3-pip-pyc): [corresponding source](https://github.com/pypa/pip/tree/26.1.2), [Alpine build source](https://github.com/alpinelinux/aports/tree/6af15ce4a424837b45da43d8dcaf1e735b25209b/main/py3-pip). The py3-pip archive contains the pip license and the licenses of the vendored packages in `pip-26.1.2.dist-info/licenses/`.
