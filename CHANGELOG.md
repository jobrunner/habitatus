# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Versions and entries below the Unreleased section are cut by release-please
from Conventional Commits — do not edit them by hand.

## [0.3.0](https://github.com/jobrunner/habitatus/compare/v0.2.0...v0.3.0) (2026-09-17)


### Features

* allow a wildcard over the subdomains of a host in the CORS allowlist ([acb3648](https://github.com/jobrunner/habitatus/commit/acb364849bf7d7ab8e78157d958f47235ecbc765))
* allow a wildcard over the subdomains of a host in the CORS allowlist ([36c5e22](https://github.com/jobrunner/habitatus/commit/36c5e2213514b9b9d9213b44a07c67a7b24fb24b))


### Bug Fixes

* canonicalise an allowlist entry before comparing it to an Origin header ([53cae56](https://github.com/jobrunner/habitatus/commit/53cae566001a4180ae1256ebc1aaef614a9b94f2))
* canonicalise dotted and numeric origin hosts ([8fdbded](https://github.com/jobrunner/habitatus/commit/8fdbded9cebcefdbb9aa026047dbb45abdce01a8))
* refuse the remaining entry spellings a browser would rewrite ([e889ab2](https://github.com/jobrunner/habitatus/commit/e889ab2d33fc5ddff0a48f5ce56b567e62280015))
* reject wildcard patterns over IPv4 literals ([3b795b8](https://github.com/jobrunner/habitatus/commit/3b795b853667a59185eb64bb84290281c3abd79a))

## [0.2.0](https://github.com/jobrunner/habitatus/compare/v0.1.1...v0.2.0) (2026-09-15)


### Features

* a healthcheck the container can actually run ([8060494](https://github.com/jobrunner/habitatus/commit/8060494df45ea2e2a804620f2e6850a3b5582354))
* a healthcheck the container can actually run ([6d3489b](https://github.com/jobrunner/habitatus/commit/6d3489b0d75438c62bb58668699a7369b660974c))


### Bug Fixes

* -mcp was reported unhealthy forever ([cac7e1e](https://github.com/jobrunner/habitatus/commit/cac7e1e50d7115dad5d705ab24ef370837ac7bc9))
* five ways the healthcheck could report the wrong thing ([f2b5df0](https://github.com/jobrunner/habitatus/commit/f2b5df0d048a8f7f9071558748df49de20255420))

## [0.1.1](https://github.com/jobrunner/habitatus/compare/v0.1.0...v0.1.1) (2026-09-14)


### Documentation

* record what the v0.1.0 release actually produced ([557b898](https://github.com/jobrunner/habitatus/commit/557b898aadbbac6a00d429fbcd10a85b1837accb))

## 0.1.0 (2026-09-14)


### Features

* a compose file for deploying on a Docker host ([daa14ce](https://github.com/jobrunner/habitatus/commit/daa14ce2b99dc0e8a0e617b1c2886663cf196f65))
* add optional CORS, and rename the compose files ([9faa12f](https://github.com/jobrunner/habitatus/commit/9faa12fcc4f772e9d9027a253aad73c23895b478))
* **ci:** guard the ratchet configuration against being weakened ([dbcdfb3](https://github.com/jobrunner/habitatus/commit/dbcdfb3fd12164103af2541946e00c7e1bf4148a))
* **classify:** request validation and the classification use case ([35b637d](https://github.com/jobrunner/habitatus/commit/35b637d388aa83a8664e35b706a058e47ec91690))
* **esy:** evaluate all ten membership condition kinds ([ae80e2b](https://github.com/jobrunner/habitatus/commit/ae80e2b5995d63415d518a6a0e5985fdf99b02bc))
* **esy:** evaluate formulas and resolve the winner per v1.2 ([0383f11](https://github.com/jobrunner/habitatus/commit/0383f11ad041540889e27886f96785570c9c5c7e))
* **esy:** three-valued logic and Jennings-Fischer cover union ([c1fbacd](https://github.com/jobrunner/habitatus/commit/c1fbacd34473e96e7acd00c4d85786f03e313a75))
* habitatus — a verified Go port of the ESy expert system ([b971cb2](https://github.com/jobrunner/habitatus/commit/b971cb28c3294ae050420e97194b40f9a9c111ec))
* **http:** POST /api/v1/classify and readiness endpoint ([858eae7](https://github.com/jobrunner/habitatus/commit/858eae7c927322dc72f24391bccb61f3d862ad56))
* initial scaffolding and habitatus design spec ([aa711ac](https://github.com/jobrunner/habitatus/commit/aa711ac47cd94cd8763d87e770ea7b70629a92b7))
* load backbone tables, expose stats and the truncation marker ([22e61eb](https://github.com/jobrunner/habitatus/commit/22e61eb601bba4c230c861ccd15f3b43d4697db0))
* **mcp:** expose classify as an MCP tool over stdio ([b972f26](https://github.com/jobrunner/habitatus/commit/b972f2645ec207c4b4053df21c185d94fc99875e))
* **rulepack:** parse formulas with R operator precedence ([7747a8d](https://github.com/jobrunner/habitatus/commit/7747a8ddeec23a9cdab8016ecf90514450c7de9e))
* **rulepack:** parse membership expressions incl. unions, EXCEPT and NON ([440b70b](https://github.com/jobrunner/habitatus/commit/440b70b7d67efa6d24f780c7c34f222e35c2fb66))
* **rulepack:** parse section 1 aggregation, first entry wins ([ac738a5](https://github.com/jobrunner/habitatus/commit/ac738a5e4fdc75bed1807ebe50997fc9fd3b453b))
* **rulepack:** parse section 2 species groups ([e99caf2](https://github.com/jobrunner/habitatus/commit/e99caf22b45428963cafb65e3d3882fae1c517bb))
* **rulepack:** parse section 3 rule headers and multi-line formulas ([fe77a82](https://github.com/jobrunner/habitatus/commit/fe77a8220869a117e330cca7316afbfc8ea0d963))
* **rulepack:** split ESy files into their four sections ([3dcb179](https://github.com/jobrunner/habitatus/commit/3dcb1793cfc0f927c7ee2fb96ec79c4ab7f65137))
* **taxa:** two-stage single-pass name resolution with cover merging ([ff67614](https://github.com/jobrunner/habitatus/commit/ff67614f330e86dba78bf5ad313818e64747f333))
* two evaluation semantics, repaired by default ([2cb974f](https://github.com/jobrunner/habitatus/commit/2cb974f60cd621f931dc567bed643eb8e235b76c))


### Bug Fixes

* "https://:443" was accepted as an origin ([dd1715a](https://github.com/jobrunner/habitatus/commit/dd1715ae2b81d75c3442a0476c4c8b96bbacc1b0))
* a bare "#" origin, and a shape check that ran too late ([cac3d59](https://github.com/jobrunner/habitatus/commit/cac3d59675ecc67782462c24f12f72b81d247cfa))
* a new package with no tests passed the coverage gate ([cad9f84](https://github.com/jobrunner/habitatus/commit/cad9f84a0293c74555790cb9c79de938773a0d76))
* a non-object attributes map escaped the exit-2 contract ([fceccca](https://github.com/jobrunner/habitatus/commit/fcecccaeb2ae71922911825c5c4a61dc775da656))
* a skipped gate reported success, and a manual release published nothing ([f3d0b35](https://github.com/jobrunner/habitatus/commit/f3d0b35faf2458b9e698039f24e3d3b88ab10eca))
* a source file with no metric escaped both complexity caps ([ccc29d5](https://github.com/jobrunner/habitatus/commit/ccc29d51d91e364588c0ecdd646b19e61bda2056))
* a swallowed error, an unlicensed module, an unlisted commit type ([b6d1c58](https://github.com/jobrunner/habitatus/commit/b6d1c5898bfc9ddd4688769bbb8874487e9b1d0f))
* a typo in the floors file could switch the ratchet off ([3533e21](https://github.com/jobrunner/habitatus/commit/3533e21195bde2cda24d384d64b37a95c98f4a49))
* **adapters:** reject malformed JSON-RPC and multi-document HTTP bodies ([19f99d6](https://github.com/jobrunner/habitatus/commit/19f99d6fec4f7423ecc0f558d3da9f6a9a81d727))
* an exemption and an unresolvable ref both walked past the guard ([0b106ab](https://github.com/jobrunner/habitatus/commit/0b106abcfb3645ed0d3d33d4040b8046a51cdc66))
* bind the examples to loopback, and keep the deploy tag in step ([67f8b4f](https://github.com/jobrunner/habitatus/commit/67f8b4fccdf555c68e31e74a54b50db159e23cf0))
* **ci:** a green check that could never go red ([7f832c3](https://github.com/jobrunner/habitatus/commit/7f832c37eace5b1c212028478ccc495358438d20))
* **ci:** a guard the PR can edit, and a digest the PR can rewrite ([7fe0c51](https://github.com/jobrunner/habitatus/commit/7fe0c51c8a14c957a67eb738621df8455e5698ed))
* **ci:** attest the released index, not the candidates ([7af0aaa](https://github.com/jobrunner/habitatus/commit/7af0aaa1d54f82b3a3220a3470d62dcfc41a8c51))
* **ci:** budget fuzzing in executions, not seconds ([edd8780](https://github.com/jobrunner/habitatus/commit/edd87806281392d486c2d5515e07c3fb215fe826))
* **ci:** pipefail everywhere, and drop the pipeline I had just introduced ([c1425e2](https://github.com/jobrunner/habitatus/commit/c1425e21d0f295be9e9dae87eb41cca7e314523c))
* **ci:** shellcheck never ran, and gofmt could fail unnoticed ([20fb150](https://github.com/jobrunner/habitatus/commit/20fb150683e1f2e6a9f41fb731951cfa50290d84))
* **ci:** stop interpolating values into release and benchmark scripts ([e59ac74](https://github.com/jobrunner/habitatus/commit/e59ac74c53bf6e3842aaa0a20e352e7f6d05e458))
* **classify:** distinguish caller errors from internal ones at the adapters ([a30b462](https://github.com/jobrunner/habitatus/commit/a30b46275f175eeecde0f0920a2bc223045eef8b))
* **classify:** reject non-finite cover and header values, bound altitude ([22c899b](https://github.com/jobrunner/habitatus/commit/22c899b8b45cf8e230fdba44ad30136df61de2c2))
* **classify:** repair CSV quoting at the source, make the country loader strict ([fe2ca85](https://github.com/jobrunner/habitatus/commit/fe2ca85a6fd67ab847d65399592ce1123630b57f))
* **esy:** correct #$$, #T$ EXCEPT, NON and NA handling per R v1.2 ([b9c84ad](https://github.com/jobrunner/habitatus/commit/b9c84adbc5203dc2fd89abd82a0b5ccd192aa438))
* **esy:** correct AND to commutative Kleene logic, verified against R ([5ca4490](https://github.com/jobrunner/habitatus/commit/5ca4490eff17014863fa77acb6487fe993f54ca7))
* **esy:** five divergences from R v1.2 found by running the upstream code ([0e23c97](https://github.com/jobrunner/habitatus/commit/0e23c97470735fece3fc6bba9bc0c471161848ef))
* **esy:** guard against stale fixtures, decide $$C on the header schema ([6c26af2](https://github.com/jobrunner/habitatus/commit/6c26af2b6463fb07ba025919d2b9cede0812ee74))
* **http:** cap request body size and stop leaking internal types ([3735ffa](https://github.com/jobrunner/habitatus/commit/3735ffad7de501a7c3a8c6573161b8eb6b05fa39))
* make the first release 0.1.0, the way the skill prescribes ([5936367](https://github.com/jobrunner/habitatus/commit/593636759834cd572ddcfca452538defaa8ff404))
* make the first release 0.1.0, the way the skill prescribes ([9ac48d5](https://github.com/jobrunner/habitatus/commit/9ac48d50f04f15dd18dd532bc0b274dbbc8d79e3))
* **mcpapi:** distinguish a bare notification from no request at all ([0377173](https://github.com/jobrunner/habitatus/commit/03771732a24c3dac772a143243bdce83dd474e2c))
* **mcpapi:** report the build version, not a literal ([406d906](https://github.com/jobrunner/habitatus/commit/406d90629bc762d3a8a77e96facf78de1e6bc429))
* **mcp:** single source of truth for header vocabularies, notification handling ([28f31b5](https://github.com/jobrunner/habitatus/commit/28f31b512777dc27bbac50a87d7253c7a7c920dc))
* narrow the secret-scan allowlist to named files ([3ebf925](https://github.com/jobrunner/habitatus/commit/3ebf92534efec818ecaf5ac77c878255efb3ffd4))
* non-finite values slipped through the text ratchet parsers ([f81fb3b](https://github.com/jobrunner/habitatus/commit/f81fb3b35a8075cb3e19e91de6448112ad40fc40))
* pin reachability on the real file, fix null metrics field, keep backbone parse issues ([57c2829](https://github.com/jobrunner/habitatus/commit/57c2829c918ff2be341b7d26805018a45a44e7b5))
* reject non-finite numbers and malformed maps in the ratchet ([a533422](https://github.com/jobrunner/habitatus/commit/a533422a988f56ea8f831ed79705f2eba889123e))
* reject origins that are not scheme://host[:port] ([95ad309](https://github.com/jobrunner/habitatus/commit/95ad309cccb3bb2a9229f9b6176574f03951a03e))
* release-please never updated the VERSION file ([eae8071](https://github.com/jobrunner/habitatus/commit/eae807140da94cc07f0b2a7a7dbe8169cd5069e8))
* release-please never updated the VERSION file ([375705d](https://github.com/jobrunner/habitatus/commit/375705d16c5535105ec7ce2dab23d3f151e8c612))
* repair a stale ESY_FILE-gated assertion, close the gate that hid it ([83b334f](https://github.com/jobrunner/habitatus/commit/83b334f1328779c24a2621520a4db0f364361f35))
* report a rule-pack identity in versions, not a server path ([d625c42](https://github.com/jobrunner/habitatus/commit/d625c42c2213a3873702b964639e414fd93d11c4))
* **rulepack:** apply header dash-stripping only to headers in trimEntry ([a281d0c](https://github.com/jobrunner/habitatus/commit/a281d0c8ca7f387e24c15de07014a6c97c0553be))
* **rulepack:** leave the glued-operator expression unsplit, as upstream does ([b5ad6bf](https://github.com/jobrunner/habitatus/commit/b5ad6bf2fe1d8a30b49c4f89fa9fad49ff1a2124))
* **rulepack:** require the sections a file must have before loading it ([40fbc5d](https://github.com/jobrunner/habitatus/commit/40fbc5d1e67a04fa5441d1ac51b7075fc977d250))
* **rulepack:** trim section-1 entries exactly as R's trim.trailing does ([9f18cf8](https://github.com/jobrunner/habitatus/commit/9f18cf8b171187164146fd0f3eaa0a8c4c66743e))
* **security:** bind to loopback by default ([ef99974](https://github.com/jobrunner/habitatus/commit/ef9997485ce06b56b505243be918c421de248519))
* **security:** close three holes in the harness itself ([baf1495](https://github.com/jobrunner/habitatus/commit/baf1495ea8203dff210049ed626c656855900dfa))
* **security:** move off the unsupported Go 1.24 line ([d6f8edc](https://github.com/jobrunner/habitatus/commit/d6f8edc451e90b6707ba593a2a189d22fb0a5f62))
* **spike:** normalise the header column names to the rule file's field names ([17fa9ed](https://github.com/jobrunner/habitatus/commit/17fa9edb620a9186509841994b02446c3354f17c))
* the exemption check fired on a file that has no baseline ([5b30ab2](https://github.com/jobrunner/habitatus/commit/5b30ab2f2b26b81d6827604d0b7806bb0bb598e3))
* the hotspot gate could pass while checking nothing ([cc7c491](https://github.com/jobrunner/habitatus/commit/cc7c491deec77c0b66c9aa3959f14b7e932bced3))
* the mutation thresholds were outside the ratchet guard ([1dee1b6](https://github.com/jobrunner/habitatus/commit/1dee1b62dec21e7a6814cedd794639ac77cbaf3c))
* three gates that measured the wrong thing, and one hot-path allocation ([e2f96ca](https://github.com/jobrunner/habitatus/commit/e2f96ca02d32f2b8c35b06b0eede334bcff66159))
* three ways past the ratchet baseline guard ([ca1f8a8](https://github.com/jobrunner/habitatus/commit/ca1f8a81876286da85f29145d22822e792415545))
* two more ways the CodeCharta ratchet could pass without judging ([d780ce7](https://github.com/jobrunner/habitatus/commit/d780ce7062829bce3a5d35297bca3c9867f54e61))
* validate the metric values, not only their presence ([a45b441](https://github.com/jobrunner/habitatus/commit/a45b441e4f4a687e0054f7d74760dd997ee401d9))

## [Unreleased]

### Added

- EUNIS habitat classification from a species list with cover values and eight
  header fields, over HTTP and MCP.
- A parser for the ESy rule file format and an evaluator verified against the
  upstream R implementation over 11,337 plots in both semantics.
- Two evaluation modes, `repaired` (default) and `faithful`.
- A hardened distroless container image with the CC BY 4.0 rule file vendored.
- The quality harness: golangci-lint with architecture gates, coverage ratchet,
  govulncheck, licence compliance, secret scanning, SBOM, mutation testing,
  fuzzing, and container scanning.

## [0.1.0]

Initial implementation.
