---
name: jig
description: Set up and run this project with jig (jig.yaml): write or extend jig.yaml for any stack (compose, Kubernetes, Go, Python, Node, Java, Rust), add infrastructure (Postgres, Redis, Kafka, RabbitMQ, OpenTelemetry), choose the UI's screens, add dashboards, queries and test suites, then start, watch and test the services. Use whenever the user asks to set up jig, add a service or component to jig.yaml, change jig's screens or panels, or run, debug or test services through jig.
---

# jig

jig runs a project's services and infrastructure (local processes, docker, kind, Kubernetes) from one
`jig.yaml`. The full key reference is `jig docs config` (MCP: `jig_docs`). Read only the section you
touch before editing: `jig docs <words>` (MCP: `jig_docs` with `search`) prints just the sections
holding them, e.g. `jig docs tasks steps`, `jig docs load generator`. Features the UI has: `jig docs guide <screen>`.

## Start a project

1. `jig init --dry-run` (MCP: `jig_init`) prints the jig.yaml jig would write from what the directory
   has: compose files, Kubernetes manifests, Go modules, Django/FastAPI/Flask, Node, Spring, Cargo.
2. Add infrastructure the code needs: `--with postgres,redis,kafka,rabbitmq,consul,prometheus,jaeger,loki,otel`.
   Look for it in the code first (connection strings, client libraries, env var names).
3. Read the draft against the code: ports each service really listens on, `depends_on`, env vars it
   reads (connection strings must point at service names: `db:5432`, not `localhost`).
4. Write it (`jig init`, or `jig_init` with `write: true`), then `jig env` must load without errors.
5. `jig up` (or `jig up --build` for kind) and `jig status`.

## One source of truth

Services already defined elsewhere are imported, never copied: `imports: [{compose: compose.yml}]`,
`[{kubernetes: deploy/k8s}]` (a service per Deployment/StatefulSet/DaemonSet; the Kubernetes runtime
deploys from those folders). jig reads them on every load, so a new workload or compose service is a
jig service at once. Under `services.<name>` write only what those files lack (a Go main to `build`,
`health`, `groups`, `run` for local processes): jig.yaml's keys merge over the imported service. An
environment `{type: docker, compose: {project: <name>, files: [...]}}` drives compose's own
containers; a remote server is `host: ssh://user@server` (or `context:`). Do not copy images,
commands, env or manifests into jig.yaml. `jig init --deep` on an existing project adds what is new
(imports included) without touching the rest.

## Edit jig.yaml

Edit the file directly (MCP: `jig_file`), then run `jig env` to check it still loads.

| want | write |
|---|---|
| a service | `services.<name>`: `build` (go: ./cmd/x, or dockerfile), `run.command` for local processes, `ports`, `env`, `env_file`, `depends_on`, `health: {port, path}` |
| infrastructure | a service with `role: infra` and `image`, plus the component that reads it: `components.db: {type: sql, driver: postgres, addr: "svc://db:5432", ...}` |
| OpenTelemetry | `otel: {endpoint: svc://otel-collector:4318}` or per signal `traces:`/`metrics:`/`logs:`; jig sets OTEL_* on every app service |
| the UI's screens | `ui: {tabs: [services, logs, metrics, traces, queries, kv, data, load, manifests, hosts, tests]}`: listed ones only, in that order; left out, every screen the project configures |
| a dashboard | `dashboards.<name>: {vars, panels: [{title, query, unit, kind, width}]}`, PromQL against a `prometheus` component |
| a saved query | `queries.<name>: {source: <component>, query: ..., every: 30s}` |
| tests | `tests.<name>`: Go `packages: [./...]`, or any runner as `command:` writing JUnit to `$JIG_JUNIT` (`pytest --junitxml=$JIG_JUNIT`) |
| per-environment changes | `environments.<env>.services.<name>: {...}` patches, `environments.<env>.otel`, `only: [...]` |

Addresses take `svc://service:port`, in components and inside env values alike (`DATABASE_URL:
postgres://app@svc://db:pg/app`): jig resolves them inside the runtime's network for containers and
pods, and forwards a port for local processes. A value used in several places (database name, user)
goes in `vars:` once (`environments.<env>.vars` per environment) and is referenced as `${NAME}`.
Secrets go in `${NAME}` with `secrets:`, never in the file.

Tasks must be idempotent: running one twice (a retry, a rerun from history, an agent) must not break
or duplicate anything. Use `IF NOT EXISTS` / `CREATE OR REPLACE`, `kubectl apply`, deletes that tolerate
missing objects (`rm -f`, `--ignore-not-found`), reset state before rebuilding it, and verify the
outcome (poll, retry, fail loudly) instead of trusting a `sleep`. Long logic goes in a script under
`scripts/` that the task calls.

## Common asks, shortest path

Read what jig already knows before searching the repository: `jig status <svc>`, `jig env`, `jig
do runtime render <svc>` (what a Kubernetes deploy applies), `jig manifests <file>` (lint one).

| ask | do |
|---|---|
| give a service a Kubernetes manifest | copy one sibling's manifest into the folder jig.yaml imports (`imports: kubernetes:`) or lists (`manifests:`), name the workload after the service, image `$REGISTRY/<image>:$TAG`; check with `jig do runtime render <svc>` |
| a new service in compose or the manifests | nothing in jig.yaml when that file is imported: `jig status` lists it; add only `build`/`health`/`groups` under its name |
| ship a change | `jig deploy -b <svc>` (build, push, roll out); the UI's `b` does the same with a tag and env to pick |
| why does X fail | `jig why <svc>` (status, health, logs, dependencies, ranked), then the code behind the first error line |
| change config a service reads | `jig kv` / the KV screen, then restart the service |
| load | `jig load run <generator> --rate N --for 1m`; `jig report <name> --since 10m` |

## Run and check

`jig status`, `jig logs -F <svc>`, `jig query <component> '<query>'`, `jig test <suite>`
(`--failed` reruns failures), `jig traces`, `jig metrics -- '<promql>'`. Mutating commands on a
protected environment need the user's go-ahead.

## When jig itself falls short

Fix jig instead of working around it in jig.yaml, when you can push to github.com/goxang/jig. Ship
every change you make there yourself: commit (Conventional Commits: `feat:` bumps the minor version,
anything else the patch), push to `main`, wait until the `ci` workflow is green; its
last job, `release`, publishes the new tag (`gh run watch`, `gh release view`). The user's only step is
`jig upgrade`; tell them the version to expect. Never leave a jig change uncommitted, unpushed or
unreleased.
