# Migrating from rig

```bash
cd my-project && jig migrate
```

With no `jig.yaml` and a `rig.yaml` here, `migrate`:

- writes `jig.yaml` from `rig.yaml`: `rig` commands in tasks, help and comments become `jig`, `.rig/`
  paths `.jig/`; `flow:` is dropped (jig has no flow screen); `compat: {rig_env: true}` keeps
  exporting `RIG_ENV`, `RIG_FILE`, `RIG_YES`, `RIG_TASK` and `RIG_ARGS` to task steps;
- copies `.rig/<env>/` to `.jig/<env>/` without the AI audit, flow settings and rebuildable `bin`,
  `run` and `logs`;
- copies `~/.config/rig` (themes, secrets, `ai.json`, sessions) to `~/.config/jig`, keeping files that
  are already there;
- pins the plugins the components, runtimes and imports name in `jig.lock`.

It never deletes or changes rig's files. Your own `${RIG_...}` variables stay as they are. On
Kubernetes, a namespace without a `jig-state` ConfigMap is read from `rig-state` and saved as
`jig-state` on its next change. `keys: rig` in `.jig/local.yaml` keeps rig's TUI keys.

## Commands

Every rig command but `flow` keeps its name, arguments and flags.

| rig | jig |
|---|---|
| `rig flow` | none: dropped |
| `rig build` | `jig build` |
| `rig deploy` | `jig deploy` |
| `rig discover` | `jig discover` |
| `rig down` | `jig down` |
| `rig exec` | `jig exec` |
| `rig logs` | `jig logs` |
| `rig push` | `jig push` |
| `rig restart` | `jig restart` |
| `rig scale` | `jig scale` |
| `rig setenv` | `jig setenv` |
| `rig start` | `jig start` |
| `rig status` | `jig status` |
| `rig stop` | `jig stop` |
| `rig up` | `jig up` |
| `rig watch` | `jig watch` |
| `rig ai` | `jig ai` |
| `rig ai check` | `jig ai check` |
| `rig ai config` | `jig ai config` |
| `rig ai connect` | `jig ai connect` |
| `rig ai log` | `jig ai log` |
| `rig ai sessions` | `jig ai sessions` |
| `rig alerts` | `jig alerts` |
| `rig attach` | `jig attach` |
| `rig debug` | `jig debug` |
| `rig ide` | `jig ide` |
| `rig metrics` | `jig metrics` |
| `rig metrics targets` | `jig metrics targets` |
| `rig profile` | `jig profile` |
| `rig report` | `jig report` |
| `rig report start` | `jig report start` |
| `rig report stop` | `jig report stop` |
| `rig test` | `jig test` |
| `rig test report` | `jig test report` |
| `rig test runs` | `jig test runs` |
| `rig traces` | `jig traces` |
| `rig cache` | `jig cache` |
| `rig data` | `jig data` |
| `rig data restore` | `jig data restore` |
| `rig data seed` | `jig data seed` |
| `rig data snapshot` | `jig data snapshot` |
| `rig data snapshots` | `jig data snapshots` |
| `rig db` | `jig db` |
| `rig db create` | `jig db create` |
| `rig db drop` | `jig db drop` |
| `rig db list` | `jig db list` |
| `rig db query` | `jig db query` |
| `rig do` | `jig do` |
| `rig kv` | `jig kv` |
| `rig kv get` | `jig kv get` |
| `rig kv ls` | `jig kv ls` |
| `rig kv put` | `jig kv put` |
| `rig kv rm` | `jig kv rm` |
| `rig load` | `jig load` |
| `rig load down` | `jig load down` |
| `rig load ls` | `jig load ls` |
| `rig load rate` | `jig load rate` |
| `rig load run` | `jig load run` |
| `rig load scale` | `jig load scale` |
| `rig load start` | `jig load start` |
| `rig load stop` | `jig load stop` |
| `rig load up` | `jig load up` |
| `rig query` | `jig query` |
| `rig queue` | `jig queue` |
| `rig queue list` | `jig queue list` |
| `rig queue publish` | `jig queue publish` |
| `rig queue purge` | `jig queue purge` |
| `rig docs` | `jig docs` |
| `rig doctor` | `jig doctor` |
| `rig env` | `jig env` |
| `rig hosts` | `jig hosts` |
| `rig infra` | `jig infra` |
| `rig infra down` | `jig infra down` |
| `rig infra restart` | `jig infra restart` |
| `rig infra status` | `jig infra status` |
| `rig infra up` | `jig infra up` |
| `rig init` | `jig init` |
| `rig kubeconfig` | `jig kubeconfig` |
| `rig manifests` | `jig manifests` |
| `rig mcp` | `jig mcp` |
| `rig migrate` | `jig migrate` |
| `rig ns` | `jig ns` |
| `rig plugins` | `jig plugin ls` |
| `rig schema` | `jig schema` |
| `rig secret` | `jig secret` |
| `rig skill` | `jig skill` |
| `rig source` | `jig source` |
| `rig task` | `jig task` |
| `rig theme` | `jig theme` |
| `rig uninstall` | `jig uninstall` |
| `rig upgrade` | `jig upgrade` |
| `rig vars` | `jig vars` |
| `rig version` | `jig version` |
| `rig why` | `jig why` |
| `rig resume` | `jig resume` |
