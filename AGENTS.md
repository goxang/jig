# AGENTS.md — jig

- Go 1.26: build and test with `GOTOOLCHAIN=local`; `gofmt`, `go vet ./...`, staticcheck and `go test -race ./...` must pass (CI runs them on Linux and macOS).
- After changing a jig.yaml type, run `go generate ./spec` (the schema test fails otherwise).
- Every step ends green and with a tmux smoke test: `tmux new-session -d -s t -x 160 -y 40 'jig'` in an example, keys sent 0.3s apart.
- Ship every change yourself: commit (Conventional Commits: `feat:` bumps the minor version, anything else the patch), push to `main`, wait for the `ci` workflow to go green (its last job, `release`, publishes the tag). The user's only step is `jig upgrade`.
- The road to 1.0 is `docs/plan.md`; follow its build order. Core grows no domains: adapters belong in `goxang/jig-plugins`.
- Working on jig itself: `skills/jig-dev/SKILL.md` (where each kind of change goes, the UI's ground rules, how to test and ship). The user-facing skill is `skills/jig/SKILL.md`; the key reference is `docs/config.md`.
