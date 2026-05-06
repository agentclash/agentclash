# Changelog

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
