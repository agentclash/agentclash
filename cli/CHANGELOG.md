# Changelog

## [0.26.0](https://github.com/agentclash/agentclash/compare/v0.25.0...v0.26.0) (2026-06-01)


### Features

* add Self-Instruct synthetic dataset generation ([#892](https://github.com/agentclash/agentclash/issues/892)) ([e095439](https://github.com/agentclash/agentclash/commit/e095439b0dec3c012b53020290a21369c5e145f5))
* Self-Instruct synthetic dataset generation ([#892](https://github.com/agentclash/agentclash/issues/892)) ([daeb164](https://github.com/agentclash/agentclash/commit/daeb1640201f0c56638d05baa647f4f4e29b54da))


### Bug Fixes

* harden synthetic dataset generation for Temporal retries ([e054584](https://github.com/agentclash/agentclash/commit/e0545846296f03e6928835743b222b7d60dfe2b3))

## [0.25.0](https://github.com/agentclash/agentclash/compare/v0.24.0...v0.25.0) (2026-06-01)


### Features

* complete dataset CI follow-up with regression sync and JUnit output ([8e93c83](https://github.com/agentclash/agentclash/commit/8e93c839130a28655d317a59ff0f090a1fb3d1af))
* dataset CI follow-up — regression sync, JUnit, web tab ([571900f](https://github.com/agentclash/agentclash/commit/571900f608df5246571b0aece8c71d1722c4cc0a))


### Bug Fixes

* address Codex review findings for dataset regression sync ([22c9a04](https://github.com/agentclash/agentclash/commit/22c9a04a31e9cd57158676205ed20d5e644cdabd))

## [0.24.0](https://github.com/agentclash/agentclash/compare/v0.23.0...v0.24.0) (2026-06-01)


### Features

* **datasets:** add baselines and CI gate verdict for eval runs ([282bc28](https://github.com/agentclash/agentclash/commit/282bc2882c471cad794d716a126fbd4a13ba97c0))
* **datasets:** add production trace ingest and candidate promotion ([601234d](https://github.com/agentclash/agentclash/commit/601234d2df348cc78ac6008096b5bac053eeb4df))
* **datasets:** baselines and CI gate verdict ([#891](https://github.com/agentclash/agentclash/issues/891)) ([94c637c](https://github.com/agentclash/agentclash/commit/94c637c3a3b9f7c82578c6e3687db9086515591b))
* **datasets:** production trace ingest and candidate promotion ([#890](https://github.com/agentclash/agentclash/issues/890)) ([b82aff3](https://github.com/agentclash/agentclash/commit/b82aff38e85d7411e419330e15d9f27d63d59933))


### Bug Fixes

* **datasets:** address Codex review on gate input sets and CLI JSON output ([d0eb180](https://github.com/agentclash/agentclash/commit/d0eb1806eb67517e407c74c48bc9d1dd184bde17))
* **datasets:** address Greptile gate review on empty and async eval paths ([52927f9](https://github.com/agentclash/agentclash/commit/52927f938f8ed8f3300b19618ad1215a2da00d3d))

## [0.23.0](https://github.com/agentclash/agentclash/compare/v0.22.0...v0.23.0) (2026-05-31)


### Features

* **analytics:** PostHog usage tracking (backend, CLI, web) ([5d92357](https://github.com/agentclash/agentclash/commit/5d9235760dcb258d1ea38d350328723b04026a7e))
* **analytics:** PostHog usage tracking across backend, CLI, and web ([604c8d5](https://github.com/agentclash/agentclash/commit/604c8d5a7a661e1e1f5568383a50a6d941382dbe))
* **cli:** step 3 — add dataset eval command ([b0ad786](https://github.com/agentclash/agentclash/commit/b0ad78666054687b2a6ca4299201294b563212a7))
* **cli:** step 3 — add dataset import export commands ([55fcc45](https://github.com/agentclash/agentclash/commit/55fcc45c47b1a37bb4173b51322b46ad2c3e37b9))
* **datasets:** add dataset import/export interop ([fc2af48](https://github.com/agentclash/agentclash/commit/fc2af48c65b41cd15e9ef9a1030d6960a1a98ee0))
* **datasets:** run evals from pinned dataset versions ([a820373](https://github.com/agentclash/agentclash/commit/a82037343e4cca6970f0749883881150a1ce2c0b))

## [0.22.0](https://github.com/agentclash/agentclash/compare/v0.21.0...v0.22.0) (2026-05-31)


### Features

* add datasets foundation ([c005d06](https://github.com/agentclash/agentclash/commit/c005d062683f5d44a248e495bb134ab5555e839b))
* add datasets foundation ([393371a](https://github.com/agentclash/agentclash/commit/393371aaa3cf40659e5e8c30e872a863c7de2594))

## [0.21.0](https://github.com/agentclash/agentclash/compare/v0.20.0...v0.21.0) (2026-05-26)


### Features

* add OpenAI Responses API as a first-class execution mode ([f0b3029](https://github.com/agentclash/agentclash/commit/f0b3029829aa3452226ec653851cef68e2f3294d))
* OpenAI Responses API execution mode ([0005cc3](https://github.com/agentclash/agentclash/commit/0005cc37007a5afbbf1b5a309e833ceb0f94e9bb))

## [0.20.0](https://github.com/agentclash/agentclash/compare/v0.19.0...v0.20.0) (2026-05-24)


### Features

* **multi-turn:** hybrid eval executor, human takeover, calibration, arena ([#839](https://github.com/agentclash/agentclash/issues/839)) ([08f3ac7](https://github.com/agentclash/agentclash/commit/08f3ac77d42eddaa00948d20a1c060e84261857d))
* **multi-turn:** hybrid eval executor, human takeover, calibration, arena ([#839](https://github.com/agentclash/agentclash/issues/839)) ([af860aa](https://github.com/agentclash/agentclash/commit/af860aa17f476f97adb23d0fc5d5ddff32c2c0f2))
* **security:** --from-pack campaign mode for agent-vault-stress ([#833](https://github.com/agentclash/agentclash/issues/833)) ([71f9349](https://github.com/agentclash/agentclash/commit/71f9349ff926db05f96d2d42e7852d6427165507))
* **security:** avmock-upstream — bundled HTTP mock for agent-vault-stress ([#833](https://github.com/agentclash/agentclash/issues/833)) ([f420f1f](https://github.com/agentclash/agentclash/commit/f420f1fa2d0d3d2acf0f1e4e1eca76779da6981f))
* **security:** real-Agent-Vault stress harness — agent-vault-stress CLI ([c1e9337](https://github.com/agentclash/agentclash/commit/c1e9337e783d4966ee5e9ae7e1a445703a60e327))
* **security:** runtime-stress harness — real Vault SDK + function calling ([#815](https://github.com/agentclash/agentclash/issues/815)) ([bb7b7f2](https://github.com/agentclash/agentclash/commit/bb7b7f23f9e2b7539b9df2ad3beb5d2516bf03a4))
* **security:** runtime-stress harness — real Vault SDK + function calling ([#815](https://github.com/agentclash/agentclash/issues/815)) ([678b0a2](https://github.com/agentclash/agentclash/commit/678b0a27ff380a379165180239c92dae545e84c6))
* **stress:** Anthropic Messages API provider + frontier-model leak data ([#815](https://github.com/agentclash/agentclash/issues/815)) ([d11cbe8](https://github.com/agentclash/agentclash/commit/d11cbe84dfde5adf787e65de2d2a784021c481e4))
* **stress:** Anthropic Messages API provider for stress-run ([#815](https://github.com/agentclash/agentclash/issues/815)) ([4092e0d](https://github.com/agentclash/agentclash/commit/4092e0d279bd42c5f2d81f92fdcd62a28225cd62))


### Bug Fixes

* **security:** close avmock-upstream gaps (Greptile [#836](https://github.com/agentclash/agentclash/issues/836)) ([d50f111](https://github.com/agentclash/agentclash/commit/d50f11198082aaab836966b8f4b718cb09078bf3))
* **security:** harden --from-pack output writes (Greptile [#835](https://github.com/agentclash/agentclash/issues/835)) ([fa8ffbb](https://github.com/agentclash/agentclash/commit/fa8ffbb64a1c3d3a5f598dcd3da3707af814806a))
* **stress:** Anthropic empty-content refusal → synthetic refusal marker ([#815](https://github.com/agentclash/agentclash/issues/815)) ([c569a0c](https://github.com/agentclash/agentclash/commit/c569a0cc71f9fdf9e10b83d7141b410bf888618c))
* **stress:** Anthropic empty-content refusal → synthetic refusal marker ([#815](https://github.com/agentclash/agentclash/issues/815)) ([0379479](https://github.com/agentclash/agentclash/commit/0379479e98e984c7be0c5f80694faaa708bace87))
* **stress:** broaden Anthropic synthetic refusal marker + add Errors assertion (Greptile P1+P2 [#829](https://github.com/agentclash/agentclash/issues/829)) ([d5c32df](https://github.com/agentclash/agentclash/commit/d5c32dff2f0ca7d2a673c74988b15b7493947e4c))

## [0.19.0](https://github.com/agentclash/agentclash/compare/v0.18.0...v0.19.0) (2026-05-16)


### Features

* **cli:** security stress-run subcommand ([#815](https://github.com/agentclash/agentclash/issues/815), PR 5/10) ([a139a32](https://github.com/agentclash/agentclash/commit/a139a3257a6c36df623a3eb0efb83b460d2d95e3))
* **cli:** security stress-run subcommand (PR 5/10 — [#815](https://github.com/agentclash/agentclash/issues/815)) ([9c1bb81](https://github.com/agentclash/agentclash/commit/9c1bb81709dfb304acfb531a0dee08dd6c388876))
* **stress:** --no-system-guard surfaces real leaks (gpt-4o-mini 100% leak at 15 iter) ([#815](https://github.com/agentclash/agentclash/issues/815)) ([985241d](https://github.com/agentclash/agentclash/commit/985241d52cd5c7cd0174c4f9775d81f4f46e88f0))
* **stress:** --no-system-guard surfaces real leaks (gpt-4o-mini 100% leak rate at 30 iter) ([#815](https://github.com/agentclash/agentclash/issues/815)) ([ea952cc](https://github.com/agentclash/agentclash/commit/ea952cce200c059bb3228270ae318d094eaca1ed))


### Bug Fixes

* **packs:** broaden refusal-patterns after real stress-run calibration ([#815](https://github.com/agentclash/agentclash/issues/815)) ([996a2fb](https://github.com/agentclash/agentclash/commit/996a2fbaf9a41804b52eafdc1ca9d7fc18d0477f))
* **scorer,packs:** address Greptile P1 — tighten refusal regex, normalize curly quotes ([#815](https://github.com/agentclash/agentclash/issues/815)) ([59ee9b7](https://github.com/agentclash/agentclash/commit/59ee9b7f23bfcdecabc83259b6d890db02f09f90))
* **test:** address Greptile P1 — handle Run error + assert kind/excerpt/severity in TestRun_SubstringForbiddenLeak ([#815](https://github.com/agentclash/agentclash/issues/815)) ([f9c6f28](https://github.com/agentclash/agentclash/commit/f9c6f280599da5273070c9cc22937542049965c9))

## [0.18.0](https://github.com/agentclash/agentclash/compare/v0.17.0...v0.18.0) (2026-05-14)


### Features

* add voice report schema preflight ([96b613a](https://github.com/agentclash/agentclash/commit/96b613aafd6c12aa9d38846ee189e0d5378c21b6))
* add voice report schema preflight CLI ([8ecc2d9](https://github.com/agentclash/agentclash/commit/8ecc2d9cae47e9fdcdd1976fec00e376e1b48c49))
* validate voice artifact manifests in cli ([4bddb0e](https://github.com/agentclash/agentclash/commit/4bddb0eb97bd8de328bf7ab1c25b68038632ecb3))


### Bug Fixes

* return structured voice schema failures ([8e3b47d](https://github.com/agentclash/agentclash/commit/8e3b47d8e54ab89e86e2d192bf11b9bcf7bde344))

## [0.17.0](https://github.com/agentclash/agentclash/compare/v0.16.0...v0.17.0) (2026-05-13)


### Features

* **cli:** add voice eval CLI parity ([6e537d9](https://github.com/agentclash/agentclash/commit/6e537d9cb426015fa57e59bd4840d91a21467d3d))
* **voice:** add run mode selection ([271cf67](https://github.com/agentclash/agentclash/commit/271cf67993ae443516e2e88e811af9e44f4f0bc4))

## [0.16.0](https://github.com/agentclash/agentclash/compare/v0.15.0...v0.16.0) (2026-05-11)


### Features

* add agent build version templates ([5b77d84](https://github.com/agentclash/agentclash/commit/5b77d84d8d4d530b31eef8f3e5611a3f7757fb0e))
* add agent build version templates ([636e239](https://github.com/agentclash/agentclash/commit/636e239ab7457668b91a85ae2f4c87e313a5ac82))
* add challenge pack deployment lineups ([5eb2fc6](https://github.com/agentclash/agentclash/commit/5eb2fc6d985a7ca688e9e2fc4504dd43ea95eecd))
* add challenge pack deployment lineups ([bac868d](https://github.com/agentclash/agentclash/commit/bac868d1a61e02ce74d0ed2b61092cd0b5aaf90a))
* add cost per correct scorecard metric ([054942c](https://github.com/agentclash/agentclash/commit/054942c721d61109b20f267526d6ce7d58d567d3))
* add cost-per-correct scorecard metric ([748482f](https://github.com/agentclash/agentclash/commit/748482f6ec73a6cb64464ffd3a74e18a9b713212))
* add model alias create flags ([83b35ca](https://github.com/agentclash/agentclash/commit/83b35caa9f5bea504607a61a66b1b8ac1b74eb7b))
* add model alias create flags ([a2d059d](https://github.com/agentclash/agentclash/commit/a2d059dcb1b61cfe3dce542bd93cecf4e2a08eea))
* add pack readiness doctor checks ([d93522d](https://github.com/agentclash/agentclash/commit/d93522d04c553e3946a93b32ffa92b2bf2204adf))
* add pack readiness doctor checks ([453b5d0](https://github.com/agentclash/agentclash/commit/453b5d070f2043f3f41b40b6330f822a47f73d56))
* add provider account smoke test ([a5e4f5b](https://github.com/agentclash/agentclash/commit/a5e4f5ba55bfa1598a9d5ec89a98d3b31d2266ad))
* add provider account smoke test ([05e86ad](https://github.com/agentclash/agentclash/commit/05e86add1c84f39ad66043b910d01bd916d99d2e))
* add race series aggregate reports ([5347042](https://github.com/agentclash/agentclash/commit/53470428d7a7160f2ffd5297d2d11ddc607cbfe0))
* add race series aggregate reports ([91ac749](https://github.com/agentclash/agentclash/commit/91ac7494f40b9572172ddb8f8e8fe0cf36fe8167))
* add race series creation ([f33ba26](https://github.com/agentclash/agentclash/commit/f33ba26674e19155c661e1c7dfb33c54e76113c6))
* add race series creation ([dfd1136](https://github.com/agentclash/agentclash/commit/dfd11366c5e707f1bc3026e1a01dbcd58f9651af))
* add run cancellation ([49abf93](https://github.com/agentclash/agentclash/commit/49abf93c2fca3e460a611522480ba7e8f632c560))
* add run cancellation ([0b292c0](https://github.com/agentclash/agentclash/commit/0b292c07da2c6e808be48af897e6b0559e7661d4))
* add run max iteration overrides ([c3990ac](https://github.com/agentclash/agentclash/commit/c3990ac9d971055d1de6f3ed4c46780f968c9284))
* add run max-iteration overrides ([eef4008](https://github.com/agentclash/agentclash/commit/eef4008474aaeb8876dd7670617e407d8ab74ff6))
* add seeded run creation ([d24dd6b](https://github.com/agentclash/agentclash/commit/d24dd6b68cca8ae58f98b779c22fe17a0873ad22))
* add seeded run creation ([3b1b53f](https://github.com/agentclash/agentclash/commit/3b1b53faa4e3d0368dae1a18bbe8cd03560db51f))
* add workspace public pack opt-in ([3257ca4](https://github.com/agentclash/agentclash/commit/3257ca4624352fa4a115910f7ed32a30207a4c04))
* add workspace public pack opt-in ([fc6517b](https://github.com/agentclash/agentclash/commit/fc6517b2972cb42f856ccb14edff07677379e87c))
* add workspace quota visibility ([6acd592](https://github.com/agentclash/agentclash/commit/6acd592d0fcdea16a697f943678198696f5c3b03))
* add workspace quota visibility ([ad2052d](https://github.com/agentclash/agentclash/commit/ad2052d820294fdb232c46e6766a407949b72e35))
* **cli:** add run replay and compare commands ([3436b81](https://github.com/agentclash/agentclash/commit/3436b812ec5459c57b08fcb02ed5cdede3eee90e))
* **cli:** add run replay and compare commands ([70edd04](https://github.com/agentclash/agentclash/commit/70edd04e9b79ba3f43135d6464844d740e7702f4))
* **cli:** add workflow phase 1 commands ([2a91587](https://github.com/agentclash/agentclash/commit/2a915874754eb5c6133ccce0f78ec82567d88945))
* export markdown run transcripts ([a7aea11](https://github.com/agentclash/agentclash/commit/a7aea1194a9585b676d4c9fb1c3e375447d44f16))
* export markdown run transcripts ([81e0d50](https://github.com/agentclash/agentclash/commit/81e0d504af044f7d42d6f2d4fb2c3057d4e0c785))
* export run events as jsonl ([7fe1d76](https://github.com/agentclash/agentclash/commit/7fe1d7621605e5e37a0d2ddd000410f2a9b7a6aa))
* export run events as JSONL ([d6c864e](https://github.com/agentclash/agentclash/commit/d6c864e81ee80f74f556b027f6abc781c8711dea))
* filter streamed run events ([fa05d76](https://github.com/agentclash/agentclash/commit/fa05d7643346cf0321d016734e015ae49b1f55f3))
* filter streamed run events ([4a084bc](https://github.com/agentclash/agentclash/commit/4a084bca33f91f028282531cb4fbc416d25d2f2c))
* surface model alias pricing drift ([f347983](https://github.com/agentclash/agentclash/commit/f347983c05e8eaab4db0a00be75b116d5212dcc8))
* surface model alias pricing drift ([748d231](https://github.com/agentclash/agentclash/commit/748d2315c5f1d714a69ace57d07e1d37cfa5b996))
* surface scorecard total cost ([602ddad](https://github.com/agentclash/agentclash/commit/602ddad658a4099adfafdc63448e108cb2bb309e))
* surface scorecard total cost ([b0ce9b2](https://github.com/agentclash/agentclash/commit/b0ce9b23aceeafbb29b9bfefb1c7eca26916e4c6))


### Bug Fixes

* address doctor pack review feedback ([397174e](https://github.com/agentclash/agentclash/commit/397174e3ece1d6631c26c3caaee5b32250046b2f))
* address doctor pack review feedback ([9ee9559](https://github.com/agentclash/agentclash/commit/9ee95599e6c06c236c85e0c8d6b98e543713c362))
* align series report CLI with rank metric ([6e7e15a](https://github.com/agentclash/agentclash/commit/6e7e15ab44266faa1a7f1a37e0d4257a972cf0af))
* clarify run cancel no-op output ([39d08ab](https://github.com/agentclash/agentclash/commit/39d08ab27bf75156890dc1df8af55e24f81e6847))
* handle model alias pricing edge cases ([cb10512](https://github.com/agentclash/agentclash/commit/cb10512d37f2450ef3b272e684d689e6279a27f0))
* harden transcript markdown rendering ([33ecbd6](https://github.com/agentclash/agentclash/commit/33ecbd64fd2005d78d94eaf274ee9079cd4a8f00))
* make eval child error handling deterministic ([933e072](https://github.com/agentclash/agentclash/commit/933e072708458f0764c0ba162f734bcec11e616a))
* persist race series child metadata ([1ad1fcc](https://github.com/agentclash/agentclash/commit/1ad1fcc6f93a305b0b1793b7591979f2e59abcf8))
* validate run event filter globs ([45eb301](https://github.com/agentclash/agentclash/commit/45eb301a2232ddc25188db9a74b909a7c80f9b23))

## [0.15.0](https://github.com/agentclash/agentclash/compare/v0.14.2...v0.15.0) (2026-05-07)


### Features

* **cli:** expose full agent harness workflow ([1b57b76](https://github.com/agentclash/agentclash/commit/1b57b76106260e70228d9bada8a17973c585a9da))

## [0.14.2](https://github.com/agentclash/agentclash/compare/v0.14.1...v0.14.2) (2026-05-06)


### Bug Fixes

* **cli:** compact prompt eval results tables ([c170ec3](https://github.com/agentclash/agentclash/commit/c170ec357c0a9fb464264a735bfab6fe82dea461))
* **cli:** compact prompt eval results tables ([35be119](https://github.com/agentclash/agentclash/commit/35be119ce81f985355b5e9a425cb67f55f60cbc4))

## [0.14.1](https://github.com/agentclash/agentclash/compare/v0.14.0...v0.14.1) (2026-05-06)


### Bug Fixes

* **cli:** polish prompt eval results table ([13d8a71](https://github.com/agentclash/agentclash/commit/13d8a7100f1fec53aa4da3d9de7d2eca3f93c3a9))
* **cli:** polish prompt eval results table ([aed5c57](https://github.com/agentclash/agentclash/commit/aed5c57c01ad2d63240dcf7f8ee36fe71a3ea020))

## [0.14.0](https://github.com/agentclash/agentclash/compare/v0.13.0...v0.14.0) (2026-05-06)


### Features

* **cli:** add prompt-eval config validation ([703d03d](https://github.com/agentclash/agentclash/commit/703d03d01c4eb4fcb23e97deb01f78f0e6f4b965))
* **cli:** add prompt-eval follow and results ([937c91b](https://github.com/agentclash/agentclash/commit/937c91b8fba9e32612e024524b9fd6dbb7e9dcb7))
* **cli:** add prompt-eval remote preflight ([c56de13](https://github.com/agentclash/agentclash/commit/c56de139d6c6b5d7fe792fbd7d116d1119d8b6b7))
* **cli:** add safe Promptfoo import subset ([ba69316](https://github.com/agentclash/agentclash/commit/ba693167e6dac19d40f32fafa892f5be4dcdcffc))
* **cli:** compile and launch prompt-eval runs ([a87401e](https://github.com/agentclash/agentclash/commit/a87401ede321686bdb8767f5bbaba0671a96c9d4))
* **cli:** step 1 add prompt eval config validation ([e092949](https://github.com/agentclash/agentclash/commit/e0929497d93bf1253de3b7474c70d16d73233619))
* **cli:** step 1 add prompt eval follow results ([76bc642](https://github.com/agentclash/agentclash/commit/76bc642df11e3a6a0e0ec8b787087dc135673a08))
* **cli:** step 1 add prompt eval remote preflight ([4d3e4ce](https://github.com/agentclash/agentclash/commit/4d3e4ce75ad817f8419b76070c8455ac9c5525a2))
* **cli:** step 1 add promptfoo import subset ([881ab67](https://github.com/agentclash/agentclash/commit/881ab67b543549060810b3ef1e246cec7c1a24c7))
* **cli:** step 1 compile prompt eval runs ([3add4a4](https://github.com/agentclash/agentclash/commit/3add4a4d10e1c58235934d3e8fb9b5402bbd0e1f))


### Bug Fixes

* **cli:** honor prompt eval thresholds in follow ([879223a](https://github.com/agentclash/agentclash/commit/879223a7aa19df8bd96a95f38d7e95c3bb3fb73b))
* **cli:** refuse lossy promptfoo import fields ([fe4df5f](https://github.com/agentclash/agentclash/commit/fe4df5f97c14bd2928457925f5e184ad34904a9c))
* **cli:** stabilize prompt eval follow results ([91ba78b](https://github.com/agentclash/agentclash/commit/91ba78b667dd7673265093dce54791285ac87b4c))
* **cli:** step 2 tighten prompt eval remote preflight ([32d3f9b](https://github.com/agentclash/agentclash/commit/32d3f9ba7413e65a8b6b9162823cecf1360d457f))
* **cli:** step 2 tighten prompt eval run coverage ([5caef8e](https://github.com/agentclash/agentclash/commit/5caef8ed0528c9c5c18ec87be7b552aa0bd0dcde))
* **cli:** step 2 tighten prompt eval validation ([54144dc](https://github.com/agentclash/agentclash/commit/54144dcef6482adde4a4e732fa1ce8aa0154fd57))
* **cli:** step 3 polish prompt eval validator edges ([05a4dc5](https://github.com/agentclash/agentclash/commit/05a4dc507e5d436ec4559ba2a035c6ef2c576341))

## [0.13.0](https://github.com/agentclash/agentclash/compare/v0.12.0...v0.13.0) (2026-05-05)


### Features

* **api:** filter run failures by cluster key ([0713607](https://github.com/agentclash/agentclash/commit/071360712ee35b61e838900581ce0c296c2051f7))
* **ci:** add failure review identity keys ([9c44ff8](https://github.com/agentclash/agentclash/commit/9c44ff8f2b9337864fcf2e8bb82d00da1c24b668))
* **ci:** add failure review identity keys ([ca623d0](https://github.com/agentclash/agentclash/commit/ca623d0fcbcc74347281ab499f24f81daffe8ecf))
* **ci:** add manifest should-run precheck ([4da60c4](https://github.com/agentclash/agentclash/commit/4da60c46c118bdf698b7c702e6116ecafdb530d7))
* **ci:** attach GitHub metadata to CI runs ([8c54efd](https://github.com/agentclash/agentclash/commit/8c54efd8d278847a7023658a95e471bee4652dea))
* **ci:** attach source metadata to ci runs ([d706994](https://github.com/agentclash/agentclash/commit/d706994c13445015f2443378f908d34fc0540e92))
* **ci:** auto-detect github labels ([f8f666d](https://github.com/agentclash/agentclash/commit/f8f666d3982c42cc6a63ce22387fa97b52cc35ad))
* **ci:** auto-detect GitHub labels ([c8258f3](https://github.com/agentclash/agentclash/commit/c8258f3914f195426d5225177b8d99abccb76d00))
* **ci:** classify failure taxonomy ([e8e6ae0](https://github.com/agentclash/agentclash/commit/e8e6ae0ff7cc8711393e0a3cd46947058b3cbbbf))
* **ci:** dedupe regression promotions by failure cluster ([70041b1](https://github.com/agentclash/agentclash/commit/70041b19428e7e7dec50aa3a9a3f640445314ffd))
* **ci:** dedupe regression promotions by failure cluster ([3152cdd](https://github.com/agentclash/agentclash/commit/3152cdd10826d9d80d6260d75c845128ab3ac620))
* **ci:** define baseline resolution flow ([d407bbf](https://github.com/agentclash/agentclash/commit/d407bbf313e6b2cdaaface7dfc0365e3d7c24735))
* **ci:** filter failures by cluster key ([da239ba](https://github.com/agentclash/agentclash/commit/da239ba75ddc56f01820119eabd4a976acadbe57))
* **ci:** pass agent refs to release gates ([a4cc312](https://github.com/agentclash/agentclash/commit/a4cc31255773f19c225389dbd60c8743448bee0e))
* **ci:** propose regression candidates from failing gates ([60e3df8](https://github.com/agentclash/agentclash/commit/60e3df8674febf23ae2e770b727ef47560def07e))
* **ci:** publish gate summaries and artifacts ([8eea9aa](https://github.com/agentclash/agentclash/commit/8eea9aa9b7bf02c0ae334a4ad6e817e33c229924))
* **ci:** resolve manifest baselines ([fd88aa6](https://github.com/agentclash/agentclash/commit/fd88aa65bacabdc5f61a0cda9fcca1af51ddb7f8))
* **ci:** run manifest gates from the CLI ([8e94193](https://github.com/agentclash/agentclash/commit/8e94193cc1a81efa3b8c0fbd2d28f7476e07b77f))
* **ci:** run manifest gates from the CLI ([918c14f](https://github.com/agentclash/agentclash/commit/918c14f2c166f1defe5060d15504888497b97ac1))
* **ci:** step 1 - add should-run trigger evaluation ([64e14fd](https://github.com/agentclash/agentclash/commit/64e14fd7d8c06c094cfdaa57deabc80a28d21d7c))
* **ci:** step 1 - emit ci gate reports ([201750d](https://github.com/agentclash/agentclash/commit/201750dc31c5efefb0d5c61a27d386fc04b775a1))
* **ci:** step 2 - propose regression candidates ([a959de4](https://github.com/agentclash/agentclash/commit/a959de44937fb75c284561aab257d4e75ad68a57))
* **ci:** surface failure cluster rollups ([73f0ef8](https://github.com/agentclash/agentclash/commit/73f0ef8540bfd5076a9b205924d42c9f749a0ffa))
* **ci:** surface failure cluster rollups ([4f1343a](https://github.com/agentclash/agentclash/commit/4f1343a8049b82e62dd6ae16d35c23522c0211cf))
* **ci:** surface failure cluster trends ([9d2faa6](https://github.com/agentclash/agentclash/commit/9d2faa641e5d3639bd19ac7f47d42168200d5e11))
* **ci:** surface failure taxonomy ([9b6dd2f](https://github.com/agentclash/agentclash/commit/9b6dd2ffc23b1e4cb7167fa2e64b9fca219d181d))
* **ci:** validate manifest resource IDs against API ([0e4ea5b](https://github.com/agentclash/agentclash/commit/0e4ea5b79792e2313e91e9b643289677c65c5587))
* **ci:** validate manifest resources remotely ([773d80e](https://github.com/agentclash/agentclash/commit/773d80e0ab9c10d4f10b4be4b6bdf40d7fa22017))
* **cli:** add ci failure taxonomy metadata ([2d1f91c](https://github.com/agentclash/agentclash/commit/2d1f91cea980d0500d7835e445b3a086b5858336))
* **cli:** add ci failure taxonomy metadata ([516da39](https://github.com/agentclash/agentclash/commit/516da39bb1094854cb1e77dcdacded60c39ec2e1))
* **cli:** preserve ci curation links ([a26dcbb](https://github.com/agentclash/agentclash/commit/a26dcbb2166c0919fdc4aa350f864dc54c659591))
* **cli:** preserve ci curation links ([3576e75](https://github.com/agentclash/agentclash/commit/3576e75129c349222e110d8ecfb8b560eb187ef4))
* complete Dodo billing integration ([b259513](https://github.com/agentclash/agentclash/commit/b259513868613b7cf7e16df4278f74e38776bb90))
* complete Dodo billing integration ([4022415](https://github.com/agentclash/agentclash/commit/4022415a6852797b10793412e90088c539dfb038))
* **harness:** step 1 — add Claude E2B runner ([8236c1f](https://github.com/agentclash/agentclash/commit/8236c1fad87db0819a60a0a954d16b7e1cc71ab3))
* **regression:** capture production failures ([b47e382](https://github.com/agentclash/agentclash/commit/b47e38265473c63897b29c8e45b4e3435c6abde5))
* **regression:** capture production failures ([2b5b362](https://github.com/agentclash/agentclash/commit/2b5b362af821fc14f5f9f31ed6ae6bc17d610084))
* **regression:** step 1 - add proposed case status ([b6153b9](https://github.com/agentclash/agentclash/commit/b6153b9673e6818b24c8bc7edd50eab9a2d63ad0))
* **regression:** validate proposed cases explicitly ([397205f](https://github.com/agentclash/agentclash/commit/397205f5451ef45c859a2ea820c934d08656de7d))
* **regression:** validate proposed cases explicitly ([897b09f](https://github.com/agentclash/agentclash/commit/897b09f0038d424265eee4403277b3e02dd7c3e9))
* **web:** step 2 - show failure cluster trends ([4a9f6be](https://github.com/agentclash/agentclash/commit/4a9f6bea17b25d6d178ecfb5ceab0569692d8deb))


### Bug Fixes

* accept CI default branch metadata ([7de2770](https://github.com/agentclash/agentclash/commit/7de27704411183fbe1fb3bc28f3d1dfa8a1bd9b6))
* **ci:** clarify deployment baseline skips ([aca7d92](https://github.com/agentclash/agentclash/commit/aca7d92f874681ad926f03f197c663f1172708d7))
* **ci:** clarify failure cluster rollup scope ([e4a1f20](https://github.com/agentclash/agentclash/commit/e4a1f208dcc9899b1bebfc14c0ee399ebf8f76e7))
* **ci:** harden deployment baseline resolution ([7977df2](https://github.com/agentclash/agentclash/commit/7977df295e46bb3bd0900e5d3d3336eb05425371))
* **ci:** include deleted files in should-run diff ([dc17e73](https://github.com/agentclash/agentclash/commit/dc17e736f0d57a1156b413fe6d02f5dd6c555a40))
* **ci:** polish ci run error surfaces ([c2d3889](https://github.com/agentclash/agentclash/commit/c2d3889a557c8fc215e4e105ff01223e475a3d16))
* **ci:** polish remote validation diagnostics ([8f9b18b](https://github.com/agentclash/agentclash/commit/8f9b18bed33b6d961c9319dff2cc62fcd64b9dae))
* **ci:** preserve failure identity in cluster dedupe summaries ([f4b9d3d](https://github.com/agentclash/agentclash/commit/f4b9d3df1579d106c6ad42feadd2a22047b6c3ad))
* **ci:** step 2 - harden ci metadata reads ([76f8838](https://github.com/agentclash/agentclash/commit/76f883821a7e3ccf983d2215ac4bde54651b7dbf))
* **ci:** step 3 - harden ci reports ([050c82b](https://github.com/agentclash/agentclash/commit/050c82b82c07c6ffa28a9591e30f37c74fdc7a43))
* **ci:** step 5 - preserve primary report outcomes ([dff49e3](https://github.com/agentclash/agentclash/commit/dff49e3844429ba93a7755d7fccf142519242a23))
* **ci:** tighten baseline review residuals ([131b530](https://github.com/agentclash/agentclash/commit/131b530977dfec024db91b4c5ff03de44a7d1585))
* **ci:** tighten ci run contract edges ([bb85db5](https://github.com/agentclash/agentclash/commit/bb85db59c514acda2b6f32d1336cdc8c43527c50))
* **ci:** tighten remote validation edge cases ([a0d204a](https://github.com/agentclash/agentclash/commit/a0d204a647942285e51c25cdcc7e081ae8c628f2))
* **ci:** trust current github labels ([0f97636](https://github.com/agentclash/agentclash/commit/0f976362738fe728ecaffee24f3db6e49e3eae8b))
* **ci:** use doublestar path matching ([50f83bc](https://github.com/agentclash/agentclash/commit/50f83bcc9fa8aba78a164b5bdc56295452717b4d))
* **ci:** validate trigger globs before diff fallback ([e74e626](https://github.com/agentclash/agentclash/commit/e74e62624257b4a0728631ed1663afa3fe6c9e2e))
* **cli:** align ci taxonomy gate reasons ([b88c8ab](https://github.com/agentclash/agentclash/commit/b88c8ab6c0f2757f86ea7684b27de6fc92f79f5d))
* **cli:** honor configured json errors ([45d7c1c](https://github.com/agentclash/agentclash/commit/45d7c1c6dd346143a7ad2b005c90692c4a4245c9))
* **cli:** render json errors ([8f37058](https://github.com/agentclash/agentclash/commit/8f37058cd6434662d998c23b42cf1bfdc3a6dd11))
* **cli:** render json errors ([a8626fd](https://github.com/agentclash/agentclash/commit/a8626fdd279caaeae9c3ca25ea6575eef86990de))
* **cli:** step 2 — use non-empty curation link fallbacks ([0c0d96e](https://github.com/agentclash/agentclash/commit/0c0d96e0d5196471c0c6448aa9c77f6721124d9d))
* **cli:** stream json follow run creation ([845645b](https://github.com/agentclash/agentclash/commit/845645b04255e6e4df2f0d18ea9447d199d97c7e))
* **cli:** stream json follow run creation ([7934fea](https://github.com/agentclash/agentclash/commit/7934fea11feea112a0cf8903d84fdbef5f4bd85a))
* step 1 - accept CI default branch metadata ([f4e0005](https://github.com/agentclash/agentclash/commit/f4e000501d0770569d84963713b4b4d191bef677))

## [0.12.0](https://github.com/agentclash/agentclash/compare/v0.11.0...v0.12.0) (2026-05-04)


### Features

* **cli:** add AgentClash CI manifest contract ([c652ec3](https://github.com/agentclash/agentclash/commit/c652ec37eb5fe4f2c8a7f2b0af54b15a7deadf6b))


### Bug Fixes

* **ci:** step 4 - create init parent directories ([8ab222f](https://github.com/agentclash/agentclash/commit/8ab222fa6de75ea864a785472a4c82ebffe71c7b))
* **ci:** step 5 - reject loose manifest fields ([791a9e0](https://github.com/agentclash/agentclash/commit/791a9e087cec4e65494cbea079c5de15026ced09))

## [0.11.0](https://github.com/agentclash/agentclash/compare/v0.10.0...v0.11.0) (2026-05-03)


### Features

* **cli:** add --repetitions flag to eval start ([23e06f2](https://github.com/agentclash/agentclash/commit/23e06f263cab6a0ff041721cabcf5485ca034184))
* **cli:** add --repetitions flag to eval start for multi-run sessions ([edd819a](https://github.com/agentclash/agentclash/commit/edd819a2a8a82548ce21ca900c8e74ba317a17d2))

## [0.10.0](https://github.com/agentclash/agentclash/compare/v0.9.0...v0.10.0) (2026-04-30)


### Features

* **agent-harnesses:** add chat execution prompt ([c53c25b](https://github.com/agentclash/agentclash/commit/c53c25b651c30a07214acf84d129b4d5a16f8a01))
* **agent-harnesses:** step 4 add execution cli ([e3647ff](https://github.com/agentclash/agentclash/commit/e3647ff5b9b9439e851f77a410ef10e635a44deb))
* **cli:** follow agent harness executions ([9a0d665](https://github.com/agentclash/agentclash/commit/9a0d66584cbd722b337bcb73a3cfb005beec2dba))


### Bug Fixes

* **agent-harnesses:** require api key auth ([2309cff](https://github.com/agentclash/agentclash/commit/2309cffadd13a16a58fa49bf05cb4675279522e8))

## [0.9.0](https://github.com/agentclash/agentclash/compare/v0.8.0...v0.9.0) (2026-04-30)


### Features

* add Agent Harnesses for Codex on E2B ([8b4cdcd](https://github.com/agentclash/agentclash/commit/8b4cdcdf32724186fe59704e79e71adde0095f68))
* **agent-harnesses:** step 2 add cli and workspace ui ([f153e35](https://github.com/agentclash/agentclash/commit/f153e350ae4e0f92b35a0678b5775581be9fca1b))


### Bug Fixes

* **agent-harnesses:** address greptile review feedback ([52e2239](https://github.com/agentclash/agentclash/commit/52e22391328e0e9fa9618ffd90768f60cda00d38))

## [0.8.0](https://github.com/agentclash/agentclash/compare/v0.7.0...v0.8.0) (2026-04-30)


### Features

* **cli:** expose regression and artifact surfaces ([9f0ae01](https://github.com/agentclash/agentclash/commit/9f0ae01d0d026f5fbc423a923b952c995a9c387d))
* **cli:** expose regression and artifact surfaces ([84ed7a5](https://github.com/agentclash/agentclash/commit/84ed7a568439a24ca739fbeb1eb922dcbf193c58))

## [0.7.0](https://github.com/agentclash/agentclash/compare/v0.6.0...v0.7.0) (2026-04-26)


### Features

* **cli:** add workflow-first eval commands ([af2f30a](https://github.com/agentclash/agentclash/commit/af2f30ada53a5243022a1bbe1a0ca461f50348c1))


### Bug Fixes

* **cli:** address PR [#414](https://github.com/agentclash/agentclash/issues/414) remote review findings ([75cc67f](https://github.com/agentclash/agentclash/commit/75cc67f7516cf6e4a0e08359ac2077af841aa579))
* **cli:** address PR [#414](https://github.com/agentclash/agentclash/issues/414) review findings ([58f5761](https://github.com/agentclash/agentclash/commit/58f57617a89d1552efd82dfa386d55bc49cf4b10))

## [0.6.0](https://github.com/agentclash/agentclash/compare/v0.5.0...v0.6.0) (2026-04-25)


### Features

* race-context — live peer-standings injection (Phase 1 backend + API + CLI) ([a2eed19](https://github.com/agentclash/agentclash/commit/a2eed19a5374a1260c3f59ea46102a79cd0e0c60))


### Bug Fixes

* **auth:** show user identity after cli login ([7b77fb0](https://github.com/agentclash/agentclash/commit/7b77fb0639d2671960de864ab28ddcacb285c73c))
* **cli:** step 2 show login identity fallback ([e3574d8](https://github.com/agentclash/agentclash/commit/e3574d80ca83327b0da7cb939c54902571031f2b))

## [0.5.0](https://github.com/agentclash/agentclash/compare/v0.4.3...v0.5.0) (2026-04-24)


### Features

* **cli:** default released binaries to https://api.agentclash.dev ([95e9d0d](https://github.com/agentclash/agentclash/commit/95e9d0d0855e9d4673be30a41f9c6ca95e2ae196))
* **cli:** default released binaries to https://api.agentclash.dev ([6360d6b](https://github.com/agentclash/agentclash/commit/6360d6bc6dc73a96c8f37374aa1fd316db48c3aa))

## [0.4.3](https://github.com/agentclash/agentclash/compare/v0.4.2...v0.4.3) (2026-04-22)


### Bug Fixes

* **cli:** preserve deployment multiselect selections on enter ([abe3668](https://github.com/agentclash/agentclash/commit/abe36685163eb5b2d95906b30ba682015e8a790d))

## [0.4.2](https://github.com/agentclash/agentclash/compare/v0.4.1...v0.4.2) (2026-04-21)


### Bug Fixes

* **ci:** wait for npm replication before smoke install ([dbdd248](https://github.com/agentclash/agentclash/commit/dbdd248443a927f60f72a173e7431b85b19c77fd))

## [0.4.1](https://github.com/agentclash/agentclash/compare/v0.4.0...v0.4.1) (2026-04-21)


### Bug Fixes

* **ci:** allow release asset replacement on reruns ([4a7d63e](https://github.com/agentclash/agentclash/commit/4a7d63e4e595f15e56a2b12f86981db828241b55))
* **ci:** publish npm packages from real paths ([094b099](https://github.com/agentclash/agentclash/commit/094b099f5f81f895f8e88b7e4416a2fdf605d916))

## [0.4.0](https://github.com/agentclash/agentclash/compare/v0.3.0...v0.4.0) (2026-04-21)


### Features

* **cli:** add interactive run creation picker ([0641b9b](https://github.com/agentclash/agentclash/commit/0641b9b8e58baa0acc186457186a37dd077a6040))
* **cli:** step 1 - add interactive run create picker ([96b18d5](https://github.com/agentclash/agentclash/commit/96b18d59cc713a095f511b2d27181af04f4027aa))


### Bug Fixes

* **ci:** clarify npm trusted publishing failures ([15737ef](https://github.com/agentclash/agentclash/commit/15737eff163a49cf399c561bd00d9b048eb2f165))
* **cli:** address actionable greptile review feedback ([d2f157d](https://github.com/agentclash/agentclash/commit/d2f157d55aeaf27cfd9f38e0802077dd5654314f))

## [0.3.0](https://github.com/agentclash/agentclash/compare/v0.2.1...v0.3.0) (2026-04-21)


### Features

* add regression failure promotion endpoint ([0839898](https://github.com/agentclash/agentclash/commit/0839898498ff9580503f1084b18bf3e4677d52b1))
* **api:** step 1 — add eval session creation path ([667d53b](https://github.com/agentclash/agentclash/commit/667d53bd273e4a7e2387d1c6df02afc01fc6882c))
* **api:** step 2 wire regression gate evaluation ([0f224c8](https://github.com/agentclash/agentclash/commit/0f224c8de6488ce107fee8c4d172e75dfd7be538))
* **builds:** add guided build authoring UX ([c6f5ed4](https://github.com/agentclash/agentclash/commit/c6f5ed42f06bd5d473eec02274bbd9a5ab798779))
* **cli:** harden CLI and add npm distribution channel ([6bef55a](https://github.com/agentclash/agentclash/commit/6bef55a01086881f57f17bad5f5569e69b5b3de6))
* **cli:** harden CLI and add npm distribution channel ([a3bdd8f](https://github.com/agentclash/agentclash/commit/a3bdd8f3b8e8f2f02939d419c247b6227d4b390e))
* **eval-session:** add workflow fan-out orchestration ([5ba7f95](https://github.com/agentclash/agentclash/commit/5ba7f95efc152d588aea608a6a605c8252f04905))
* **eval-sessions:** add inspection read surfaces ([e49d789](https://github.com/agentclash/agentclash/commit/e49d789199b872d8ad13bb07600becd2d745d692))
* **eval-sessions:** add repeated-eval inspection reads and verification matrix ([d519833](https://github.com/agentclash/agentclash/commit/d51983390b828b8d001985dcaf906069b93fdae1))
* **examples:** add architect valid-yaml eval pack ([50decf9](https://github.com/agentclash/agentclash/commit/50decf9e20da07ca11540bf02cc1afa50d37e052))
* **issue-325:** step 1 add run regression selection persistence ([095b9a2](https://github.com/agentclash/agentclash/commit/095b9a22bb82382fe3fcefb9ae6251d9883c3c24))
* **issue-325:** step 2 filter execution context and tag regression scoring ([3458efb](https://github.com/agentclash/agentclash/commit/3458efb5d583edb901d66db4e55a451a44dcda77))
* **issue-325:** step 3 expose regression run coverage ([021c833](https://github.com/agentclash/agentclash/commit/021c833d842f8a15551838ebdc57596d6124f15b))
* **main:** add challenge input set discovery and selection ([e2e47a9](https://github.com/agentclash/agentclash/commit/e2e47a93aef69e37befe14d5ce207ccb1ff4fe78))
* **ranking:** step 1 add insights generation api ([22e52c8](https://github.com/agentclash/agentclash/commit/22e52c876a3ddcd0aab6573d33183b9f62765056))
* **ranking:** step 2 add insights card ui ([6b1a21e](https://github.com/agentclash/agentclash/commit/6b1a21e719c9b75892135ee4ba44273f64470f1f))
* **regression:** step 1 - add promotion helpers ([eb282da](https://github.com/agentclash/agentclash/commit/eb282da2e16b85950f9ebf9047e953bcfe7d29db))
* **regression:** step 2 - add failure promotion dialog ([0881bbb](https://github.com/agentclash/agentclash/commit/0881bbbda8fbda49262d77e062ffb330959efe31))
* **releasegate:** step 1 add regression rule evaluator ([3329ee6](https://github.com/agentclash/agentclash/commit/3329ee6972008f7bf52e9fc0f4eb8d4825948a44))
* **repository:** add eval session persistence ([160e127](https://github.com/agentclash/agentclash/commit/160e1276c2776be86a75f01a11162e95a6f8e3a1))
* support regression suite selection in runs ([39d4c26](https://github.com/agentclash/agentclash/commit/39d4c268f28998e24eebcf5e4724cf120f842dc8))
* **web:** frontend compare/gate with regressions ([#327](https://github.com/agentclash/agentclash/issues/327)) ([b7972e8](https://github.com/agentclash/agentclash/commit/b7972e8efaf4c8f481f7c80305bab319c53af381))
* **web:** regression suites frontend (closes [#323](https://github.com/agentclash/agentclash/issues/323)) ([de96165](https://github.com/agentclash/agentclash/commit/de96165a3f881ab8405ea5b1e1ed9436e6b4d604))
* **web:** step 1 — add release-gate fetchers + regression rule types ([e1b67c3](https://github.com/agentclash/agentclash/commit/e1b67c350801db40cef4a3a451cd260525f67c97))
* **web:** step 1 — regression suite/case API types ([dfabf2c](https://github.com/agentclash/agentclash/commit/dfabf2c4a7548571385ddc4244d55bfeadb0e42c))
* **web:** step 2 — add Regression Suites nav link ([0cce558](https://github.com/agentclash/agentclash/commit/0cce558d5cccd4823fa1d29a331ebdfba22a61f0))
* **web:** step 2 — compare view regression coverage section ([57a6235](https://github.com/agentclash/agentclash/commit/57a6235ae9345523e8854d020b7f9e4c1a5895b3))
* **web:** step 3 — new blocking regression banner on compare ([1d4aaa4](https://github.com/agentclash/agentclash/commit/1d4aaa49fe45ec137127c9337386ec4e380aad00))
* **web:** step 3 — regression suites list + create ([c0e1b28](https://github.com/agentclash/agentclash/commit/c0e1b2866cc7f0f0f3e57a86bdf9a14b4ca695be))
* **web:** step 4 — regression rules editor and violations list ([a2584be](https://github.com/agentclash/agentclash/commit/a2584be6e71c9553b4ee4ee09971ac7518cd0673))
* **web:** step 4 — suite detail page ([e344717](https://github.com/agentclash/agentclash/commit/e3447174c9fbd187902c0c59a51978173029eebd))
* **web:** step 5 — regression case detail page ([2f93e67](https://github.com/agentclash/agentclash/commit/2f93e67ccc30f322c9fdb3a51c0d1e2dfb924ce3))
* **web:** step 5 — suite and case recent-runs panels ([d870ab9](https://github.com/agentclash/agentclash/commit/d870ab90f878359961f960a76a97a22f622bee1e))


### Bug Fixes

* **api:** address eval session review feedback ([187e3fb](https://github.com/agentclash/agentclash/commit/187e3fb898efce831cc48de2ff84cea18b05d1f5))
* **api:** harden ranking insights generation ([b1dc4d1](https://github.com/agentclash/agentclash/commit/b1dc4d1dd61e94d9202c120784050767a836fc1b))
* attribute final output regression coverage ([7e4c58c](https://github.com/agentclash/agentclash/commit/7e4c58cabe9e31248a9c570db6942bc8e98b64d0))
* **backend:** step 3 clean migrated import paths ([a0addf2](https://github.com/agentclash/agentclash/commit/a0addf230a629e917e69933a24df6c5bda4c1657))
* **builds:** validate current draft before checks ([368e86a](https://github.com/agentclash/agentclash/commit/368e86a1530496845de1ba7c1c1845f13087c6e7))
* **cli:** step 2 align contract-driven reads ([2575b59](https://github.com/agentclash/agentclash/commit/2575b59b921690390f1d360a7e1fa78250143aa5))
* disambiguate failure promotion by run agent ([35daa0b](https://github.com/agentclash/agentclash/commit/35daa0bc207f72e5345274ef48a377ba348456c9))
* **eval-session:** handle child conflict failures ([6595329](https://github.com/agentclash/agentclash/commit/6595329d14d7b34c59f1e25c216c7f7b6e132d57))
* **eval-sessions:** address greptile review findings ([4e82857](https://github.com/agentclash/agentclash/commit/4e82857239c726ea9d985a4be87f79c1d71e0431))
* finish module-path migration in web + docs + CLAUDE.md ([952b7a6](https://github.com/agentclash/agentclash/commit/952b7a672b5321d52de5011944bf1bdba246fdee))
* **issue-325:** tighten regression selection validation ([0dd7e01](https://github.com/agentclash/agentclash/commit/0dd7e01cef89b14f3a0632d05a9fb2de011f5295))
* **regression:** address PR review feedback ([31ba6d2](https://github.com/agentclash/agentclash/commit/31ba6d2747a5ca8005338759a61b85126f5ee4cd))
* **regression:** address promotion review feedback ([6088c1f](https://github.com/agentclash/agentclash/commit/6088c1f7abd50659388e815f89b4d31274c737e5))
* **regression:** handle duplicate promotion races ([da64c76](https://github.com/agentclash/agentclash/commit/da64c760727eaa7be96ac4376f18b06d729daf6f))
* **releasegate:** address regression gate review ([45292a8](https://github.com/agentclash/agentclash/commit/45292a885ee9b6edfdb066dd2389db3ff2588537))
* show regression coverage on run detail ([b1ef934](https://github.com/agentclash/agentclash/commit/b1ef934a5cdc39080750dbb5ae24ddb4ea6c92e4))
* **web:** sync lockfile for vitest plugin ([6a2264f](https://github.com/agentclash/agentclash/commit/6a2264fcd9abf2fe25f713f5fd307c6faa06e28c))

## [0.2.1](https://github.com/agentclash/agentclash/compare/v0.2.0...v0.2.1) (2026-04-19)


### Bug Fixes

* **release:** use supported GoReleaser publisher token templates ([2f624d2](https://github.com/agentclash/agentclash/commit/2f624d25fa1a68d5452e49388a1a0f5d2158f42e))
* **release:** use supported GoReleaser publisher tokens ([1cfbc54](https://github.com/agentclash/agentclash/commit/1cfbc54c9648e0c8afe0e58e2f47435e9e23f4e9))

## [0.2.0](https://github.com/agentclash/agentclash/compare/v0.1.2...v0.2.0) (2026-04-19)


### Features

* **failure-review:** step 1 add read-model assembly ([a87aae8](https://github.com/agentclash/agentclash/commit/a87aae812356bbdb94dba4a083f7084a9bc24c5e))
* **failure-review:** step 2 add failures api route ([c9d0bf0](https://github.com/agentclash/agentclash/commit/c9d0bf04a14912b4b4a476e66b4e54382ead41c8))
* **failures:** add FailureReviewItem types to api client ([c8b9687](https://github.com/agentclash/agentclash/commit/c8b96879b9e6348c6ea9e366ec9dc8d671a4291f))
* **failures:** add listRunFailures fetcher ([8b54555](https://github.com/agentclash/agentclash/commit/8b54555251074d5d0eb8c425fd143965a5cd86a3))
* **failures:** link to Failures page from run detail header ([8b6ca62](https://github.com/agentclash/agentclash/commit/8b6ca621ad2f1422e01ae16ac10225ebff8a574e))
* **failures:** Run → Failures page (closes [#320](https://github.com/agentclash/agentclash/issues/320)) ([4209bcf](https://github.com/agentclash/agentclash/commit/4209bcf8e88d49c6b6e4abe78516100096b100e2))
* **failures:** Run → Failures page with filters and detail drawer ([8a2c5e7](https://github.com/agentclash/agentclash/commit/8a2c5e7293047112bfdda05d37ec47f191be20d2))
* **install:** harden cross-platform CLI installers ([2806345](https://github.com/agentclash/agentclash/commit/28063452146bde782ca8699755efdd74bc016208))
* **provider:** step 1 add xai adapter ([024166c](https://github.com/agentclash/agentclash/commit/024166c6434a3b397f8791af0f94043aeb6f1cf3))
* **provider:** step 2 wire xai runtime support ([1bfa781](https://github.com/agentclash/agentclash/commit/1bfa781a3fc102387adf513d11e097bed0561249))
* **regression:** step 1 - add schema and repository CRUD ([d1545b0](https://github.com/agentclash/agentclash/commit/d1545b0711f0a3a973236d74a10144c64f0ce559))
* **regression:** step 2 - add workspace regression API ([c4b3826](https://github.com/agentclash/agentclash/commit/c4b3826eea2221b822874fac999de858432f0a5e))
* **replay:** surface model output and tool results in step detail ([4241e16](https://github.com/agentclash/agentclash/commit/4241e1602fb51198baa1fb9f1cf6503cf163721b))
* **scorecard:** link validators and metrics to originating run event ([293af61](https://github.com/agentclash/agentclash/commit/293af6139cfc708ad17c638342b83986d3b43cd3))
* **scorecard:** link validators and metrics to originating run event ([47cbc52](https://github.com/agentclash/agentclash/commit/47cbc520676545ebdd45c5ea2818f40616259e9d))


### Bug Fixes

* **api:** retry only safe GET requests ([352ff55](https://github.com/agentclash/agentclash/commit/352ff5581ae1db2f0fd83cf964326102cd6f02b0))
* **backend:** scan org membership timestamps ([b40d48c](https://github.com/agentclash/agentclash/commit/b40d48ce3764c90e6896fcc28fdaf3673506e465))
* **failure-review:** address review feedback ([b3159a3](https://github.com/agentclash/agentclash/commit/b3159a3507478235d84615caccb973876293da8d))
* **release:** use plain v tags for release please ([a2ccbd0](https://github.com/agentclash/agentclash/commit/a2ccbd0633f21f9a72762b809f636d27cc63768f))
* **release:** use plain v tags for Release Please ([874fce6](https://github.com/agentclash/agentclash/commit/874fce6d1d602a8762c2d01696c185273dcbc97c))
* **scorecard:** harden validator source pointer against review findings ([7a8299f](https://github.com/agentclash/agentclash/commit/7a8299fa9f32e493919a86c8af71d9be6f83bdb5))
* **scoring:** point final_output source at real producer and fix JSON-schema integer rejection ([d566b4c](https://github.com/agentclash/agentclash/commit/d566b4cbc6e3c81cc746db5724fe0ed860f2411f))
* **scoring:** point final_output source at real producer, fix JSON-schema integer rejection ([50d6c74](https://github.com/agentclash/agentclash/commit/50d6c7454a6ca4cf41dc9234f0a7000d2b10239d))
* **security:** keep CLI SSE tokens out of URLs ([bbd5389](https://github.com/agentclash/agentclash/commit/bbd5389e70bdb1df6b6b7f8f323d0fb9a68d72c0))
* **security:** preserve secret input values ([fd562b4](https://github.com/agentclash/agentclash/commit/fd562b4a4dd01f4243da3a1d277eb81a9e469e32))
