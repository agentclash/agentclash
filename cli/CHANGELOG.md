# Changelog

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
