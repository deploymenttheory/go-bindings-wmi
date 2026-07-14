# Contributing

Thanks for your interest in improving this project.

## Issues

Please file bugs and feature requests on the
[issue tracker](https://github.com/deploymenttheory/go-bindings-wmi/issues), filling out
the template. Small, well-described reports are genuinely useful contributions.

## Pull requests

- PR titles follow [Conventional Commits](https://www.conventionalcommits.org/)
  (`feat:`, `fix:`, `docs:`, `chore:`, …) — this is CI-enforced.
- Run `gofmt`/`go vet` and keep the build and tests green.
- By contributing you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Generated code

**Never hand-edit generated code under `bindings/`** — it is produced by
`cmd/generate` from the committed schema snapshot (`metadata/cim/*.json`) and
overwritten on every run; CI diffs a fresh regenerate. Fix the generator
(`cmd/generate`) or the hand-written runtime (`runtime/wmi`) and regenerate.
Capturing a new schema snapshot (`cmd/capture`) is a separate, deliberate act.
