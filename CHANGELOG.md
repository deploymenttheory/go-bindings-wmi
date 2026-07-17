# Changelog

## 0.1.0 (2026-07-17)


### Features

* bootstrap the WMI/CIM bindings — capture, generate, runtime, v0 cimv2 ([cae9fcf](https://github.com/deploymenttheory/go-bindings-wmi/commit/cae9fcfdab929a85f85af7a651dde19baf7c28e2))
* capture all of root\cimv2 (~1,300 classes), coerce numeric VARIANTs; mature governance/docs ([e7e1bcf](https://github.com/deploymenttheory/go-bindings-wmi/commit/e7e1bcf90d2492a739b9f51fd78b2524f797d635))
* constants and edges ([5394287](https://github.com/deploymenttheory/go-bindings-wmi/commit/5394287dcf2ef7065e7dffcd46581e157cc65bfc))
* csp lcrud ([71320a3](https://github.com/deploymenttheory/go-bindings-wmi/commit/71320a32762dfc6457c52a4cc64f36adb9fac650))
* **csp:** committed DDF v2 snapshots (Feb 2026) and generated bindings ([a992d35](https://github.com/deploymenttheory/go-bindings-wmi/commit/a992d352b9340ed1c57ed670cbdbf1d05160c9fb))
* **csp:** DDF v2 schema, parser, and runtime descriptor types ([ca6c880](https://github.com/deploymenttheory/go-bindings-wmi/commit/ca6c880308f8055b347eb7e8fd4f87af002318a9))
* **csp:** extend the bridge join to non-Policy CSPs (Phase 4 breadth) ([46deb44](https://github.com/deploymenttheory/go-bindings-wmi/commit/46deb44a6868a875b1eb9688281451ea093a6ca0))
* **csp:** fetchddf acquisition stage and gencsp generator ([aaf971f](https://github.com/deploymenttheory/go-bindings-wmi/commit/aaf971f7e8e041a23a026067bcab6f133f2b9531))
* **csp:** join DDF policies to the bridge and add a Read/Set/Delete runtime ([7639f5e](https://github.com/deploymenttheory/go-bindings-wmi/commit/7639f5e79dfa28995ee6c90311d3f842f6921d89))
* **csp:** regenerate bindings with bridge mappings ([1efaadd](https://github.com/deploymenttheory/go-bindings-wmi/commit/1efaaddd6b50c9d323f30af667fa19fe707abab0))
* ddf csp pipeline ([d8ad89f](https://github.com/deploymenttheory/go-bindings-wmi/commit/d8ad89f01f1721ddd17b1904f71ec03b1e5045d9))
* embedded objects completion ([cceb7b3](https://github.com/deploymenttheory/go-bindings-wmi/commit/cceb7b3948a76190d03ffdd9ae5e1231b166cc0d))
* enhance WMI Row handling and schema updates ([84d5d70](https://github.com/deploymenttheory/go-bindings-wmi/commit/84d5d70a4770f256e0bc00f72196c78432bc0737))
* enhance WMI Row handling and schema updates ([487d0be](https://github.com/deploymenttheory/go-bindings-wmi/commit/487d0be7429dc3398eed33ba348eca2dfc17bf05))
* **examples:** csp-lcrud inspect verb — discover bridge key conventions ([c878847](https://github.com/deploymenttheory/go-bindings-wmi/commit/c878847fe9bbe58d1b11d29d4d4f228fffa44063))
* **generate:** emit &lt;ns&gt;_constants.go from enumeration qualifiers ([007b5e8](https://github.com/deploymenttheory/go-bindings-wmi/commit/007b5e89399cd6bc6ec6623e4e2eac7deeb54337))
* **generate:** Get&lt;Class&gt; key lookups from the captured [key] qualifiers ([0261f5b](https://github.com/deploymenttheory/go-bindings-wmi/commit/0261f5b99910f110b1584d7fad8d6d2f695ab21d))
* **generate:** split emitted packages by construct, like the winmd sisters ([b85151e](https://github.com/deploymenttheory/go-bindings-wmi/commit/b85151e7d3661ad6860b82d244857c2c4c18d445))
* **generate:** type embedded objects as wmi.Row and references as string ([e29bab1](https://github.com/deploymenttheory/go-bindings-wmi/commit/e29bab10b3e25892f76650dd83901f08c7039bd0))
* instance crud ([b3a5bc1](https://github.com/deploymenttheory/go-bindings-wmi/commit/b3a5bc1d6927401e04b324a1619e0ab616639544))
* mdm bridge ([82adaa8](https://github.com/deploymenttheory/go-bindings-wmi/commit/82adaa8f93ae5be312643fe2eb11c8bc2302c7b2))
* **mdm:** capture root\cimv2\mdm\dmmap and generate bindings/cim/dmmap ([9529675](https://github.com/deploymenttheory/go-bindings-wmi/commit/9529675f76b0895a04406cf9ee1d5e4278f32de4))
* **mdm:** SYSTEM-capture tooling and docs for the root\cimv2\mdm\dmmap bridge ([22c5e81](https://github.com/deploymenttheory/go-bindings-wmi/commit/22c5e8181b2cb1c674145418d44898020c3cd47e))
* **metadata:** recapture root\cimv2 with keys+methods; add StandardCimv2 and SecurityCenter2 ([54bc4a8](https://github.com/deploymenttheory/go-bindings-wmi/commit/54bc4a842243aea336fed3ad2ffa2e550d4b7932))
* **pipeline:** validate + diff subcommands, method capture, golden tests ([ac81cbe](https://github.com/deploymenttheory/go-bindings-wmi/commit/ac81cbeb2d435bf9b835de3ac25ac3967f3caaad))
* prep for v0.1.0 ([6ef4430](https://github.com/deploymenttheory/go-bindings-wmi/commit/6ef4430beb07fd5ff60f22202240c5d30693e487))
* **runtime:** complete the VARIANT data plane and grow the domain surface ([ff06bb6](https://github.com/deploymenttheory/go-bindings-wmi/commit/ff06bb60d0f1d657aa00f710b288570c8dc2c040))
* **runtime:** decode and encode embedded CIM objects; encode all slice widths ([c03b752](https://github.com/deploymenttheory/go-bindings-wmi/commit/c03b7528b04786ccb8b54846048c2bc4fd0269d8))
* **runtime:** enum qualifier capture, []wmi.Row encoding, ExecMethodContext, remote creds ([8127e04](https://github.com/deploymenttheory/go-bindings-wmi/commit/8127e0499e02574f16cfd5caa9e3e4af0d387129))
* **runtime:** instance CRUD, association traversal, WQL literal helpers ([8c5aef1](https://github.com/deploymenttheory/go-bindings-wmi/commit/8c5aef13f74d8cbb2915607e8d062b45224295e8))


### Bug Fixes

* **ci:** let include:scope derive the dependabot commit scope ([8ef307a](https://github.com/deploymenttheory/go-bindings-wmi/commit/8ef307ad1d63d9dd938d47381383d27422743f4f))
* **ci:** stop dependabot doubling the commit scope to chore(deps)(deps) ([0ed238d](https://github.com/deploymenttheory/go-bindings-wmi/commit/0ed238d1379bee6c5f86902920aff522fb234596))
* **csp:** scope-relative bridge ParentID and query-based execution ([a35d9f0](https://github.com/deploymenttheory/go-bindings-wmi/commit/a35d9f04f4a756eacf9373ec1f4d76792360f887))
* **docs:** add a complete Related projects section to the README ([7ad1ec5](https://github.com/deploymenttheory/go-bindings-wmi/commit/7ad1ec51248628767a2b7f3967f962d426e46393))
* **mdm:** make Capture-MdmBridge.ps1 pure ASCII for Windows PowerShell 5.1 ([d2f0334](https://github.com/deploymenttheory/go-bindings-wmi/commit/d2f033432e29bc15022774cb136faf03e7340f77))
* **mdm:** run capture via Register-ScheduledTask; keep the script pure ASCII ([b5b5710](https://github.com/deploymenttheory/go-bindings-wmi/commit/b5b571042cff432d2e4700153ad8dbc93bd1172c))

## Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Releases and their notes are managed automatically by release-please.
