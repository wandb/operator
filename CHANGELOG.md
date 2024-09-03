# Changelog

All notable changes to this project will be documented in this file.

### [1.12.5](https://github.com/wandb/operator/compare/v1.12.4...v1.12.5) (2024-09-03)


### Bug Fixes

* Debug logging errors ([#26](https://github.com/wandb/operator/issues/26)) ([a641621](https://github.com/wandb/operator/commit/a64162125994b9c43f5253f3fb9f097773740bc7))

### [1.12.4](https://github.com/wandb/operator/compare/v1.12.3...v1.12.4) (2024-09-03)


### Bug Fixes

* Log the diff of specs ([#23](https://github.com/wandb/operator/issues/23)) ([c0ea0d8](https://github.com/wandb/operator/commit/c0ea0d8c4e4c3375ff0765c980952c3cbb064df9))

### [1.12.3](https://github.com/wandb/operator/compare/v1.12.2...v1.12.3) (2024-09-03)


### Bug Fixes

* Debugging logic ([#22](https://github.com/wandb/operator/issues/22)) ([2c019b8](https://github.com/wandb/operator/commit/2c019b83850c5eca246d5aace74e05f370cf0004))

### [1.12.2](https://github.com/wandb/operator/compare/v1.12.1...v1.12.2) (2024-09-03)


### Bug Fixes

* Debug logging the cache ([#21](https://github.com/wandb/operator/issues/21)) ([26e8fd5](https://github.com/wandb/operator/commit/26e8fd56b1a42eb5bbdf9d5af28f56a17c60232e))

### [1.12.1](https://github.com/wandb/operator/compare/v1.12.0...v1.12.1) (2024-08-26)


### Bug Fixes

* Support Openshift permissions schema for the helm cache ([#17](https://github.com/wandb/operator/issues/17)) ([b498f79](https://github.com/wandb/operator/commit/b498f79a439464fab8d14eb76cc8ead499a80208))

## [1.12.0](https://github.com/wandb/operator/compare/v1.11.2...v1.12.0) (2024-07-29)


### Features

* Allow the operator to support installation without cluster level permissions ([#16](https://github.com/wandb/operator/issues/16)) ([6f29a3e](https://github.com/wandb/operator/commit/6f29a3e436b0d7a467d0ce457de985c9f1f7a3de))

### [1.11.2](https://github.com/wandb/operator/compare/v1.11.1...v1.11.2) (2024-06-27)


### Bug Fixes

* Mask sensitive values in log ([#14](https://github.com/wandb/operator/issues/14)) ([514336d](https://github.com/wandb/operator/commit/514336da58a6200753a71d219801a6da7bfc6860))

### [1.11.1](https://github.com/wandb/operator/compare/v1.11.0...v1.11.1) (2024-03-06)


### Bug Fixes

* Bump controller tools version to latest ([#13](https://github.com/wandb/operator/issues/13)) ([c52dbb6](https://github.com/wandb/operator/commit/c52dbb6f2d8524f09c8599b9343d095f8f324f3f))

## [1.11.0](https://github.com/wandb/operator/compare/v1.10.1...v1.11.0) (2024-03-06)


### Features

* **operator:** Add airgapped support ([#12](https://github.com/wandb/operator/issues/12)) ([bfd3796](https://github.com/wandb/operator/commit/bfd37965e9c8385309ecdbe566d8c914f6ea8c86))


### Bug Fixes

* add license log ([#11](https://github.com/wandb/operator/issues/11)) ([e129fab](https://github.com/wandb/operator/commit/e129fab56dfb8e04cafb5fd3839aaea33f9492ec))

### [1.10.1](https://github.com/wandb/operator/compare/v1.10.0...v1.10.1) (2023-10-18)


### Bug Fixes

* Properly merge chart specs together ([37c41bc](https://github.com/wandb/operator/commit/37c41bcb18fe5e2de67f4b19c02cc292f743adfc))

## [1.10.0](https://github.com/wandb/operator/compare/v1.9.4...v1.10.0) (2023-09-23)


### Features

* Add option to set reconcileFrequency ([484c014](https://github.com/wandb/operator/commit/484c014a950db59c37cb529952233123548de3b5))

### [1.9.4](https://github.com/wandb/operator/compare/v1.9.3...v1.9.4) (2023-09-02)


### Bug Fixes

* Correct merge order ([cd49cef](https://github.com/wandb/operator/commit/cd49cefdf248293bb1ffd95c52edab6bfdf284be))

### [1.9.3](https://github.com/wandb/operator/compare/v1.9.2...v1.9.3) (2023-09-01)


### Bug Fixes

* Assign metadata instead of merging it ([908c839](https://github.com/wandb/operator/commit/908c83959d57efff1d3b5fd5e29221239ba96218))

### [1.9.2](https://github.com/wandb/operator/compare/v1.9.1...v1.9.2) (2023-09-01)


### Bug Fixes

* properly get license ([6ff6533](https://github.com/wandb/operator/commit/6ff65333d0161e3c0211aa6be3a151cc499473fa))

### [1.9.1](https://github.com/wandb/operator/compare/v1.9.0...v1.9.1) (2023-09-01)


### Bug Fixes

* Setting cached release namespace incorrectly ([e585555](https://github.com/wandb/operator/commit/e585555b888844019fa1a0995173c65fc3c4d234))

## [1.9.0](https://github.com/wandb/operator/compare/v1.8.11...v1.9.0) (2023-08-31)


### Features

* Add caching for deployer release requests ([1185b40](https://github.com/wandb/operator/commit/1185b40169004f5b05a8f04440a31cff30c30fb9))

### [1.8.11](https://github.com/wandb/operator/compare/v1.8.10...v1.8.11) (2023-08-30)


### Bug Fixes

* correctly check if chart is installed based on status ([384d330](https://github.com/wandb/operator/commit/384d3309cd9789825d699a424085d83b60ffcd74))

### [1.8.10](https://github.com/wandb/operator/compare/v1.8.9...v1.8.10) (2023-08-28)


### Bug Fixes

* Secret reading metadata ([6dab7ed](https://github.com/wandb/operator/commit/6dab7edb7ceacaaf4bd61e99a71df7ea17fbcc69))

### [1.8.9](https://github.com/wandb/operator/compare/v1.8.8...v1.8.9) (2023-08-28)


### Bug Fixes

* Save active spec metadata ([47bd862](https://github.com/wandb/operator/commit/47bd8621d7ed7faef83ae421c2c0f8fa1c902ea9))

### [1.8.8](https://github.com/wandb/operator/compare/v1.8.7...v1.8.8) (2023-08-25)


### Bug Fixes

* Channel spec not getting applied correctly ([6e763a8](https://github.com/wandb/operator/commit/6e763a8379e3d7d6c1541f0293e1a9142302150f))

### [1.8.7](https://github.com/wandb/operator/compare/v1.8.6...v1.8.7) (2023-08-24)


### Bug Fixes

* Properly update complete status ([86a5196](https://github.com/wandb/operator/commit/86a5196e04522ccb45d7f5d81f57447670de795c))

### [1.8.6](https://github.com/wandb/operator/compare/v1.8.5...v1.8.6) (2023-08-24)


### Bug Fixes

* secrets stored with correct values ([f6d61e9](https://github.com/wandb/operator/commit/f6d61e93ec52f9ecccc2737ffd6822f5e33ebd6c))

### [1.8.5](https://github.com/wandb/operator/compare/v1.8.4...v1.8.5) (2023-08-23)


### Bug Fixes

* Pass namespace into chart ([e8e0b8f](https://github.com/wandb/operator/commit/e8e0b8fa88f237c3fbbf858da2fd9333631ebec2))

### [1.8.4](https://github.com/wandb/operator/compare/v1.8.3...v1.8.4) (2023-08-23)


### Bug Fixes

* Properly set namespace for deployments ([53f51a9](https://github.com/wandb/operator/commit/53f51a9db2cd9fdc615430050c32f55f30dd7a95))

### [1.8.3](https://github.com/wandb/operator/compare/v1.8.2...v1.8.3) (2023-08-23)


### Bug Fixes

* Properly parse chart from deployer ([5eabdfe](https://github.com/wandb/operator/commit/5eabdfe600729b1046b334d1388dae34e97011f9))

### [1.8.2](https://github.com/wandb/operator/compare/v1.8.1...v1.8.2) (2023-08-22)


### Bug Fixes

* Rename config -> values and release -> chart ([519cd1b](https://github.com/wandb/operator/commit/519cd1bc225971ab4e4d0552c28184266a1bcde2))

### [1.8.1](https://github.com/wandb/operator/compare/v1.8.0...v1.8.1) (2023-08-18)


### Bug Fixes

* Charts download ([57355ce](https://github.com/wandb/operator/commit/57355ceaa2e7760941e93bf476fb2fdf09056aba))

## [1.8.0](https://github.com/wandb/operator/compare/v1.7.0...v1.8.0) (2023-08-18)


### Features

* Add support for helm repo releases ([dfef752](https://github.com/wandb/operator/commit/dfef75205de39bc08765eb2143ac6cfd9b5dca43))

## [1.7.0](https://github.com/wandb/operator/compare/v1.6.1...v1.7.0) (2023-08-17)


### Features

* use secrets instead of configmaps ([049797f](https://github.com/wandb/operator/commit/049797feafa971a07cedadf510e7b6497ac2cbfd))

### [1.6.1](https://github.com/wandb/operator/compare/v1.6.0...v1.6.1) (2023-08-17)


### Bug Fixes

* Simplify docker image ([1cf55e4](https://github.com/wandb/operator/commit/1cf55e4c926dfb4122a4ff30b4bea6161cfc8a22))

## [1.6.0](https://github.com/wandb/operator/compare/v1.5.0...v1.6.0) (2023-08-17)


### Features

* Add helm support ([077765c](https://github.com/wandb/operator/commit/077765ca9c45115dddac24508377b4d5912c160a))

## [1.5.0](https://github.com/wandb/operator/compare/v1.4.4...v1.5.0) (2023-08-12)


### Features

* Use container based deployments only ([3e6b222](https://github.com/wandb/operator/commit/3e6b22238bdfa9c6f545200e0b8b52a268d2d18d))

### [1.4.4](https://github.com/wandb/operator/compare/v1.4.3...v1.4.4) (2023-08-11)


### Bug Fixes

* Jobs work? ([9972d26](https://github.com/wandb/operator/commit/9972d26f5a1131fd3c6c26481b882cb5e447ed31))

### [1.4.3](https://github.com/wandb/operator/compare/v1.4.2...v1.4.3) (2023-08-11)


### Bug Fixes

* reorder backup ([ab66486](https://github.com/wandb/operator/commit/ab66486bddecb74bcda55e342d58ff168120905c))

### [1.4.2](https://github.com/wandb/operator/compare/v1.4.1...v1.4.2) (2023-08-11)


### Bug Fixes

* Using validate for job spec ([5c7ff66](https://github.com/wandb/operator/commit/5c7ff667506dbf0ae89096d8704c586301a2b608))

### [1.4.1](https://github.com/wandb/operator/compare/v1.4.0...v1.4.1) (2023-08-11)


### Bug Fixes

* Use cdk8s image for apply container ([189bc08](https://github.com/wandb/operator/commit/189bc08173ad0ce433b28ec8ed00a933a6916b14))

## [1.4.0](https://github.com/wandb/operator/compare/v1.3.0...v1.4.0) (2023-08-11)


### Features

* Support for deploymenting via jobs ([da801ea](https://github.com/wandb/operator/commit/da801eace75d336bf47c2b21fead504c18559ee4))

## [1.3.0](https://github.com/wandb/operator/compare/v1.2.13...v1.3.0) (2023-08-11)


### Features

* Add active-state cm ([#2](https://github.com/wandb/operator/issues/2)) ([5a6c4c3](https://github.com/wandb/operator/commit/5a6c4c355b415ad3d182644ade95e69274da5b9e))

### [1.2.13](https://github.com/wandb/operator/compare/v1.2.12...v1.2.13) (2023-08-10)


### Bug Fixes

* add operator properties to config ([b5f48f0](https://github.com/wandb/operator/commit/b5f48f0a02ab6c03006e8ecb7195d551e61ecf8e))

### [1.2.12](https://github.com/wandb/operator/compare/v1.2.11...v1.2.12) (2023-08-10)


### Bug Fixes

* merge func ([94aa0d0](https://github.com/wandb/operator/commit/94aa0d0a08e64c9f31c025f6a55dd9ba062faa07))
* remove submodule ([bdb408a](https://github.com/wandb/operator/commit/bdb408afbd75fcc32b319b6a15e63ae1018508d7))
* Rename config spec cfs ([672100a](https://github.com/wandb/operator/commit/672100a5985bc48e4daca57b11992ea2b065a8a7))
* rename configs ([8727281](https://github.com/wandb/operator/commit/8727281c3cef49fcebaa360a142e3a561a93e383))

### [1.2.11](https://github.com/wandb/operator/compare/v1.2.10...v1.2.11) (2023-08-09)


### Bug Fixes

* Git release pulls correctly ([d47aebd](https://github.com/wandb/operator/commit/d47aebd258cff8fee928b0abc40f2c3c497129b1))

### [1.2.10](https://github.com/wandb/operator/compare/v1.2.9...v1.2.10) (2023-08-09)


### Bug Fixes

* remove debugging logs ([d4da31f](https://github.com/wandb/operator/commit/d4da31f80d7ba972ccc0a77aade3da227f2a2773))

### [1.2.9](https://github.com/wandb/operator/compare/v1.2.8...v1.2.9) (2023-08-09)


### Bug Fixes

* refactor spec ([87be86b](https://github.com/wandb/operator/commit/87be86be08cbcbae67aabcb63e516c570842e1cb))

### [1.2.8](https://github.com/wandb/operator/compare/v1.2.7...v1.2.8) (2023-08-09)


### Bug Fixes

* Refactor specs ([7c6da34](https://github.com/wandb/operator/commit/7c6da34a5572407fec6e4a8164f2678ff506669e))

### [1.2.7](https://github.com/wandb/operator/compare/v1.2.6...v1.2.7) (2023-07-20)


### Bug Fixes

* x-kubernetes-preserve-unknown-fields ([bedac52](https://github.com/wandb/operator/commit/bedac525823b5d9bcb50ab8de931a8431f92eecc))

### [1.2.6](https://github.com/wandb/operator/compare/v1.2.5...v1.2.6) (2023-07-19)


### Bug Fixes

* Preserve unknown fields ([565a25f](https://github.com/wandb/operator/commit/565a25f6c4f01c5aa2321c432affb0f26a337d28))

### [1.2.5](https://github.com/wandb/operator/compare/v1.2.4...v1.2.5) (2023-07-19)


### Bug Fixes

* Default to dev mode ([d961f77](https://github.com/wandb/operator/commit/d961f77f854d72f37329bf0609c02cf9bf940a82))

### [1.2.4](https://github.com/wandb/operator/compare/v1.2.3...v1.2.4) (2023-07-19)


### Bug Fixes

* Output json format logs ([90af7b6](https://github.com/wandb/operator/commit/90af7b65a749c5456734778fbf24d398f604371f))

### [1.2.3](https://github.com/wandb/operator/compare/v1.2.2...v1.2.3) (2023-07-18)


### Bug Fixes

* Add operator namespace env ([846731a](https://github.com/wandb/operator/commit/846731a43e1abd757fb9a5ff5d4f47651d6023dc))

### [1.2.2](https://github.com/wandb/operator/compare/v1.2.1...v1.2.2) (2023-07-17)


### Bug Fixes

* Use deployer release channels ([480b380](https://github.com/wandb/operator/commit/480b38055826405b1fcf3ff3508e65d4e2cc10b3))

### [1.2.1](https://github.com/wandb/operator/compare/v1.2.0...v1.2.1) (2023-07-12)


### Bug Fixes

* Remove ui building step ([08ee985](https://github.com/wandb/operator/commit/08ee985c4fafec27d0fd8b706a8b47224a94e980))

## [1.2.0](https://github.com/wandb/operator/compare/v1.1.13...v1.2.0) (2023-07-12)


### Features

* Add events recording ([388d37b](https://github.com/wandb/operator/commit/388d37bec08eb594cdc161d6a4661ca96947bc9a))

### [1.1.13](https://github.com/wandb/operator/compare/v1.1.12...v1.1.13) (2023-07-10)


### Bug Fixes

* pass spec namespace and name ([79d77f2](https://github.com/wandb/operator/commit/79d77f2859a041bba92838992f3b706ab9bce7b1))

### [1.1.12](https://github.com/wandb/operator/compare/v1.1.11...v1.1.12) (2023-07-10)


### Bug Fixes

* remove console ([fba45ee](https://github.com/wandb/operator/commit/fba45ee9235cbdc72b6013b86b275a8012eb8aa7))

### [1.1.11](https://github.com/wandb/operator/compare/v1.1.10...v1.1.11) (2023-07-10)


### Bug Fixes

* set namespace when running kubectl apply ([1d6f00c](https://github.com/wandb/operator/commit/1d6f00cd33f87b98b7c8c92c84ffecf19d9facc3))

### [1.1.10](https://github.com/wandb/operator/compare/v1.1.9...v1.1.10) (2023-07-10)


### Bug Fixes

* kubectl not working in docker image ([ffc694e](https://github.com/wandb/operator/commit/ffc694e3d56975c79567ac34ae7e135a5872601c))

### [1.1.9](https://github.com/wandb/operator/compare/v1.1.8...v1.1.9) (2023-07-09)


### Bug Fixes

* Install kubectl in docker image ([e5df9de](https://github.com/wandb/operator/commit/e5df9de6cc61335e22cfd6c185b4b0b5c926bada))

### [1.1.8](https://github.com/wandb/operator/compare/v1.1.7...v1.1.8) (2023-07-09)


### Bug Fixes

* docker build ([d160a9c](https://github.com/wandb/operator/commit/d160a9c5aa43fc8bff9dc2d3488367aa4b897827))

### [1.1.7](https://github.com/wandb/operator/compare/v1.1.6...v1.1.7) (2023-07-09)


### Bug Fixes

* lock pnpm version ([c2608f7](https://github.com/wandb/operator/commit/c2608f766056a93bc5621be9b46ea68e1cc5f589))

### [1.1.6](https://github.com/wandb/operator/compare/v1.1.5...v1.1.6) (2023-07-05)


### Bug Fixes

* Tmp directory permissions ([b0820f5](https://github.com/wandb/operator/commit/b0820f515e6d3f186da3307058e6f194afb7334b))

### [1.1.5](https://github.com/wandb/operator/compare/v1.1.4...v1.1.5) (2023-07-05)


### Bug Fixes

* Clean up docker image ([ef7c629](https://github.com/wandb/operator/commit/ef7c62922dc583c788c913167ef39304fe7d2b90))

### [1.1.4](https://github.com/wandb/operator/compare/v1.1.3...v1.1.4) (2023-07-05)


### Bug Fixes

* Add debugging for installing release ([893ebd9](https://github.com/wandb/operator/commit/893ebd99c585ad1f49e9f2275a7f3f6b28a1f36f))

### [1.1.3](https://github.com/wandb/operator/compare/v1.1.2...v1.1.3) (2023-06-30)


### Bug Fixes

* add applied config to download bundle ([bef77c2](https://github.com/wandb/operator/commit/bef77c2b03f81b3b4ea096dee547b00ea53eacf3))

### [1.1.2](https://github.com/wandb/operator/compare/v1.1.1...v1.1.2) (2023-06-30)


### Bug Fixes

* Serve console with gin ([c9e04aa](https://github.com/wandb/operator/commit/c9e04aad530e95570fdf9a83b68228cf44904cac))

### [1.1.1](https://github.com/wandb/operator/compare/v1.1.0...v1.1.1) (2023-06-30)


### Bug Fixes

* Add console namespace and service name to config properties ([0b9efef](https://github.com/wandb/operator/commit/0b9efef1a47e53ec1352e1a350ca7b53ed1a2b8b))

## [1.1.0](https://github.com/wandb/operator/compare/v1.0.8...v1.1.0) (2023-06-30)


### Features

* Add support for release from a git repository ([8a6b073](https://github.com/wandb/operator/commit/8a6b07375f4885a767ba5688166d69c158d4abc9))

### [1.0.8](https://github.com/wandb/operator/compare/v1.0.7...v1.0.8) (2023-06-29)


### Bug Fixes

* add pnpm, node and git to docker image ([176b6f0](https://github.com/wandb/operator/commit/176b6f083369b3929b7cc2574dd30a54aae6f5b2))

### [1.0.7](https://github.com/wandb/operator/compare/v1.0.6...v1.0.7) (2023-06-28)


### Bug Fixes

* rename versioning step name ([77bf4ed](https://github.com/wandb/operator/commit/77bf4ed30fc3ae729c6fe33ae4e35d4d3226caa7))

### [1.0.6](https://github.com/wandb/operator/compare/v1.0.5...v1.0.6) (2023-06-28)


### Bug Fixes

* upgrade semantic to v3 ([594c463](https://github.com/wandb/operator/commit/594c463187dde87150abdca81b0135f650e5eccc))

### [1.0.5](https://github.com/wandb/operator/compare/v1.0.4...v1.0.5) (2023-06-28)


### Bug Fixes

* push images to dockerhub ([d4cdd27](https://github.com/wandb/operator/commit/d4cdd27519f02444af9dc81b3080ce49d20bd75f))

### [1.0.4](https://github.com/wandb/operator/compare/v1.0.3...v1.0.4) (2023-06-28)


### Bug Fixes

* install go version ([6664b4b](https://github.com/wandb/operator/commit/6664b4b1aa3d6e807c51d080b9fa858cdb452ee9))

### [1.0.3](https://github.com/wandb/operator/compare/v1.0.2...v1.0.3) (2023-06-28)


### Bug Fixes

* docker image push ([e08b3da](https://github.com/wandb/operator/commit/e08b3da814e2b3656b6f5cd888930d581e32d243))

### [1.0.2](https://github.com/wandb/operator/compare/v1.0.1...v1.0.2) (2023-06-28)


### Bug Fixes

* clean up env for image push ([7213ed2](https://github.com/wandb/operator/commit/7213ed23aa67eb3a221d4d58a133eabcaa533012))

### [1.0.1](https://github.com/wandb/operator/compare/v1.0.0...v1.0.1) (2023-06-28)


### Bug Fixes

* rename docker variables ([274e20c](https://github.com/wandb/operator/commit/274e20cc8e8bff9f0d856b3b41528051ea8eae0c))

## 1.0.0 (2023-06-28)


### Bug Fixes

* add gh token for ci ([72d456f](https://github.com/wandb/operator/commit/72d456fb9c4ce0709d45c6306f9f8496c3edd9f3))
* added changelog commits ([61b5f5d](https://github.com/wandb/operator/commit/61b5f5d2383cf341acbe188662643ef387e2bf07))
* Create release rc files ([f7f4622](https://github.com/wandb/operator/commit/f7f46221ec4b5b7eae0726b4c96205652a139814))
* init controller ([0f0a9e9](https://github.com/wandb/operator/commit/0f0a9e98fadd103c6d61afe23c87e7247d27ccc8))
* revert to v2 for semver ([535a721](https://github.com/wandb/operator/commit/535a7215eea0a26ed19c6a7febda817208abd9f4))
