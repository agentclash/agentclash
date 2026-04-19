# Changelog

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
