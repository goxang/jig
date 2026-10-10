# jig guide

Everything the [README](../README.md) leaves out: the CLI, every screen of the terminal UI, the AI
assistant, GoLand and agents. Every jig.yaml key is in [config.md](config.md).

## CLI

```bash
jig                           # terminal UI
jig infra up                  # infrastructure, once
jig up test                   # every service tagged test, and what they depend on
jig up --build                # build images and deploy everything, phase by phase
jig build app -t v1 --ref main && jig deploy app -t v1   # apps only; infrastructure untouched
jig build app -t v1 --no-push && jig push app -t v1      # build locally, push later
jig scale core +1             # every service in group core, one more replica
jig status
jig -e kind logs -F api worker
jig query                     # saved queries and queryable components
jig query db-top-cpu n=5
jig load rate fleet 200 && jig load scale fleet +2
jig task bootstrap            # a named list of steps from jig.yaml (jig task <tab> completes)
jig watch api                 # rebuild and restart as the sources change (ctrl+w in the UI)
jig why checkout is slow      # incident: logs, traces, metrics, deploys; the AI's root cause and fix; answer "fix"
jig doctor                    # what this environment lacks: tools, cluster, ${VARS}, ports, components (--json)
jig vars set MAIN_DB=x        # manifest variables per environment; jig setenv api K=V for env
jig data db Switch tables     # walk databases and caches
jig data snapshot db before   # pg_dump/mysqldump/BACKUP into .jig/<env>/snapshots (S on the Data screen)
jig data restore db before    # put it back; jig data seed db runs the db's seed:
jig logs -E '(?i)timeout' api # regex over logs
jig alerts                    # what is over its thresholds
jig test unit --race          # a test suite; --failed reruns the failures, -o file saves the report
jig test ./pkg/x/...          # any packages, no suite needed
jig report load --since 20m   # metrics, traces and queries of a load test (reports: in jig.yaml): md, json or xml
jig report start load; jig test e2e; jig report stop load   # a report of exactly that run
jig load run smoke --vs last  # a load: run with its profile and assertions, compared with the last one
jig test e2e --ci -o e2e.xml  # for pipelines: plain output, JUnit; failures exit 3
jig inbox                     # writes waiting for approval: the assistant's and agents'
jig agent watch               # run agents: on their events and schedules
jig ns --create me            # Kubernetes: switch the environment to a new namespace (jig ns lists them)
jig metrics targets -o f.json # services as Prometheus file_sd targets, for a local Prometheus
jig resume last               # the TUI as a saved session (S) left it
jig ide --open                # every service into GoLand's Services view by section: live logs, stop, debug
jig attach api                # start it if down, follow its logs; Ctrl-C stops it (what GoLand's configs run)
jig mcp                       # MCP server for AI agents
jig call services.status      # any action or source by name, through policy
jig lock --for 1h             # hold the environment: nobody else changes it until then
```

`-e <env>` (or `$JIG_ENV`) picks the environment. A `protected: true` environment refuses changes
without `--yes`; pushing over an existing image tag needs it too. A `readonly: true` one refuses them
even with it. Exit codes: 1 an error, 3 a check failed (tests, load assertions, a report), 4 denied by policy
or a protected environment that needs `--yes`. In the TUI the confirmation is the `--yes`; `jig --yes` opens it already confirmed, so a
protected environment asks nothing more.

## Terminal UI

Screens load nothing until opened, and only the one showing refreshes. Everything is a sortable grid
(`ctrl+alt+←→` pick the column, `ctrl+alt+↑↓` order it; `< >`, `alt+↑↓` and `ctrl+shift+arrows` too, or click a header) and works with the mouse. On a Kubernetes
cluster, dangerous changes (stop, deploy, delete, edits) ask first and `enter` confirms; local, docker
and kind never ask. `?` shows the keys of the current screen.
Drag over any text (results, queries, test output, the chat) to copy it; the selection stays inside the
box it starts in. The chat keeps code lines unwrapped and scrolls sideways (`shift+←→`, the wheel's tilt
or shift+wheel, or by dragging past its edge). Copies go to the system clipboard (wl-copy, xclip or xsel) and over OSC 52.

| screen | |
|---|---|
| Services | your services by section (`sections:`; `[` `]` or a click, `i` infrastructure); mark with `space`, a whole section with `a` or a double-click on it, then start (in dependency order)/stop/restart/scale/deploy them together; `h` edits a Kubernetes autoscaler (or creates one), and scaling past one asks whether to move it; `R` changes requests and limits; `F` lists the service's manifests to edit, sync from what runs, or apply; replica changes (an autoscaler's too) show for a while; `D` attaches a debugger (debug build, a stable port per service); `o` puts every service into GoLand (below); `p` takes a profile (the picker says what each kind shows) as a sortable table, `W` saves it as a report; `enter` opens one: instances and its live log, which scrolls like the Logs screen, `l` moves it there |
| Logs | the services you pick, merged, or one instance. JSON, console and logfmt lines read as `LEVEL message key=value`, with JSON, protobuf text and Go `%v` values inside fields shown as compact JSON; `h` hides fields, `s` shows lines raw. `/` filters by text, regex or fields: `output.Transaction.ID=202604 level!=debug msg~timeout` (`=`, `!=`, `~` contains; terms ANDed). `↑` `↓` or a click pick a line, `enter` inspects it as a tree (`f` filters by the selected field, `w` wrap, `J` `K` next line); dragging over text copies it |
| Metrics | dashboards as tabs, `$variables` and time range in the header (click them), foldable rows; click a legend entry for only that series, ctrl-click to hide it; `v` opens a panel with a sortable legend table and a cursor, under a toolbar in sections (series, time, query); `p` on a series opens the pod (or service) its labels name in Services; drag across a chart zooms to that time range, `Z` (or ⊖) zooms out, `,` `.` (or ‹ ›) shift it, ctrl+wheel zooms around the mouse |
| Traces | filter by service, operation, minimum duration, time window, text, errors |
| Queries | saved queries (with parameters), ad hoc ones, schedules with a trend of the first number; `H` every run of the session; `y` copies the query (or, on the result, the row), `Y` the whole result as TSV; `/` on a result filters its rows like the Data screen. Writing a query opens a popup: the whole text wrapped, `ctrl+a` selects it, `ctrl+c` / `ctrl+y` copy it |
| KV | browse, edit and delete keys right in the store; `/` searches every key, field and value as you type and opens the hit with its field selected; `enter` edits a JSON value field by field; `o` picks the editor; `R` restarts the services that read the edited key; `F` loads the config files into it (`kv-*` tasks) |
| Data | databases (objects, rows, definitions, running queries), caches (keys, values), queues; `e` edits a cell, `space` marks rows, `D` deletes them (table rows by primary key, Redis keys and entries); `Q` queries where you stand (on a table row: that row by its primary key; on a Redis key: the read for its type); going back (`esc` or ‹ back) lands on the row you opened; `y`/`Y` copy the row/all as TSV; `/` filters rows: words match any cell fuzzily, `*glob*` whole cells, `Destination=Pos` (any case, `*` globs), `col!=x` and `col~text` one column (a unique part of its name will do); `@?` turns plain words into such a filter |
| Load | generators, each asked on its own so the running ones (●, selected first) show at once, with target and actual rate and error share, and the total being sent; rate and config shared by every instance, instance count, `i` (or a click) one instance's charts or all, `c` their KV config, `v` their env, `W` saves a metrics report; `z` or a click shows a chart full size, with the Metrics panel view's legend, filter, range and drag to zoom |
| Manifests | on Kubernetes it opens on what runs in the namespace (`c` switches to the files and back): field edits there apply to the cluster at once, `L` edits with kubectl, `s` writes the running object into the file that holds it; on the files: objects or folders (`t`), relations, a file's issues (`i`), apply (`a`), make a service (`n`); `→` (or a click) walks the object field by field like the KV screen: `enter` edits a value, `a` adds a field, `D` deletes one, each saved into the file at once with its comments kept; `/` searches every field and value and lands on the field; `e` edits an object in your editor and saves it into its file, `s` writes what the cluster runs into the file (only fields someone set, `$VARS` kept), `L` edits it on the cluster; `d` picks among every manifest folder of the project |
| Hosts | nodes as htop-style CPU, memory and disk bars, the selected one's CPU history, a shell (double-click), and `p` the pods on a node, sortable by CPU, memory or age |
| Tests | suites as tabs and the `go test` command they run; `r` runs, `f` reruns failures, `.` the selected test, `O` sets flags (race, cover, -run, …); a tree of packages and tests (`i` cycles failed/passed/skipped/running), its output beside it, benchmarks with the change since the last run, saved runs (`h`) |

`jig` opens on the environment it last showed (`-e` or `$JIG_ENV` picks another). Screens switch with
`1`-`0` and `` ` `` (the eleventh), `shift+←→` or `alt+←→`, or a click on their name; tabs inside a screen
(dashboards, apps/infra, objects/folders, saved/history, suites, filters) switch with `ctrl+←→` and are clickable too. A screen
shows only when jig.yaml gives it something (no `kv` component, no KV screen); `ui: { tabs: [services,
logs, data, tests] }` picks the screens and their order, and the number keys follow it.
`T` runs a task from jig.yaml in the background (`ctrl+o` on one shows its args and steps); a task keeps
running when you quit jig, and a saved session reopened later follows it again or shows how it ended. `ctrl+e` shows the environment (variables with secrets hidden, each
component's address and database; `/` searches it, `e` edits a variable), `N` switches or creates a Kubernetes namespace, and the Metrics screen's
`m` points the dashboards at another metrics source. `S` saves the session (screens, query results and history) and pins it: from then on `jig` opens on it as you left it
(`jig --fresh` starts clean and unpins; `jig resume` picks another),
under the user's config directory (`~/.config/jig/projects/...`); `M` frees the mouse so
the terminal can select text; the header shows alerts (`A`). `q q` (or `ctrl+c ctrl+c`) quits, at once unless a test run or an
operation is still going, or load generators are sending: those stop with jig (tests, queries, port
forwards) or keep going without it (generators, services), and it asks.

On the Services screen, `d` deploys a tag or a whole image (the running one is filled in), `$` edits a
service's env, `F` opens its manifests (edit the file, or the live object on the cluster), and `b` runs
any of build, push and deploy for the selected or marked services with a tag you choose, then shows the
env each service deploys with (on Kubernetes, its live workload's) to change or add any first; its
last row reaches the environment's deploy variables (database names, say). A build asks for the branch
first (the working tree, or a local or remote branch built from a clean checkout), and a ship task's
picker is the same one, groups and sections in tabs.

What you start (a build, a deploy, a task) runs in the background: the line above the keys shows the
running one and its last output line, and `!` opens the activity view with every operation of the
session, the full output of the one picked (`y` copies it, the wheel scrolls), `x` to stop it and the errors (`tab`). A
task opens that view on itself; when a step asks for input (the line above the keys says so), `enter`
there types the answer. Several tasks can run at once. A question or an input opens in its own box
above the keys, which then show the keys that box takes; `esc` on it returns to the picker or box it
came from. Opening a service shows its instances first: `enter` follows the picked one's log (or all
of them), `esc` goes back to the instances, then to the list. A jump to another screen (a service's
metrics, its logs) is walked back with `esc` or `⌫` once nothing is left to close there; with no jump
to walk back, `esc` goes home: the Services list, every service. An input left with `esc` keeps what was
typed as a draft the next time it opens. `S` saves the session (and pins it): the screen, each grid's
sort, each screen's filters (services, KV, manifests, traces, the metrics range, tests), drafts, query
history and results, the activity view and the errors come back with it; it saves again as you switch
screens and when you quit. `ctrl+r` reloads `jig.yaml` in place, keeping the filters, after you or the
assistant edited it (the status line says when the file changed). The help
(`?`) filters as you type. A border under the mouse lights up and drags to resize the panes. Errors a
new setup meets (a tool not installed, Docker not running, no Kubernetes cluster) say what to do.

`ctrl+p` picks a colour theme, previewing each as you move (`jig theme` lists and sets them too):
default, nord, dracula, gruvbox, catppuccin, clay, ember, mono, or your own. `jig theme --save mine nord` writes
`~/.config/jig/themes/mine.yaml` to edit; a theme file sets `base:` and only the colours it changes,
with `light:` for light terminals (without it, the default theme's light shades). Any colour too faint
to read on the terminal, a bar or a cursor line is lightened or darkened just enough, so a theme of
your own stays legible on dark and light terminals alike. `$JIG_THEME` overrides the choice for one
run. Fonts belong to the terminal; jig sets colours only.

A Kubernetes environment can name its cluster by address as well: `runtime: { context: local, server:
https://rancher.example/k8s/clusters/local }` uses the context called `local` when the kubeconfig has
one, else whichever context points at that server (Rancher names downloaded kubeconfigs per user).

## AI

Set it up once, in three steps:

1. Install one backend: [opencode](https://opencode.ai) (`curl -fsSL https://opencode.ai/install | bash`) or
   [Claude Code](https://claude.com/claude-code) (`npm i -g @anthropic-ai/claude-code`), and log in to it.
   No login: opencode's free models work as they are.
2. `jig ai check` says what jig found and whether a turn works. To use your own key instead,
   `jig ai config provider=openai url=https://…/v1 api_key=… model=…` (or `deepseek`, `anthropic`, `9router`).
3. Press `@` on any screen, or run `jig ai why is api failing?` in the terminal.

For an agent that edits the project (Claude Code, Codex, Cursor, opencode) instead: `jig skill --install`
teaches it jig.yaml, and the MCP config under [Agents](#agents) gives it jig's tools.

`@` on any screen opens a chat about what the screen shows: the selected service, key, table, the log
lines in view. It runs your own **opencode** or **Claude Code** (whichever is installed) with jig as its
only tools, so it can do anything you can: change a KV key, restart services, run and schedule queries,
open screens, explain logs against the code, suggest `jig profile` / `go tool pprof` commands.
Query prompts (Data `Q`, Queries, Metrics, Logs grep, KV values) get inline completions: `tab` takes them,
`ctrl+t` turns them off or on for good. A suggested query (Data `Q`, a new query) waits behind the empty
input: type your own and the AI completes it, `tab` takes the suggestion to edit, `enter` runs it as is.
`@?` then words asks for the rest in plain language: `@?logs slower than 1s`, or
`SELECT * FROM transactions WHERE @?amount above 100k`; the AI's version shows in its own
bordered "✦ ai suggests" box above the input, `tab` takes it, `enter` runs it.
`@?` works in every filter and query prompt (each label says so). Before offering an answer jig tries
it: a regex or filter must compile and keep some rows, PromQL must run, a reading SQL, LogQL, TraceQL
or KV query runs once. A failure goes back to the AI with the error, up to three tries; writes,
kubectl, HTTP and broker queries are never run to check.
Data `Q` on a procedure or function writes its call with every parameter as `NULL /* type */` to fill in,
AI or not.

The chat renders the answers' markdown (lists, tables, code) wrapped to the box, and types in its own
box under them. `enter` while the assistant works queues the message for after the answer; `ctrl+x`
stops it (queued messages come back into the input). `↑` recalls what you sent before, `pgup`/`pgdn`
(or `shift+↑↓`, the wheel) scroll, and the empty input suggests a next question: `tab` takes it.
`/new` starts a fresh conversation even mid-answer: the other keeps running, `/sessions` brings it back,
and several can run at once.

In the chat, `/` opens a menu of its commands with what each does (`↑↓` picks, `tab` completes, `enter`
runs; `/help` lists them). `/model` and `/effort` pick the chat's model (opencode's whole list, or opus/sonnet/haiku) and
reasoning level, `/fast` the completion model, `/autocomplete` switches suggestions. Completions through
the backend start it every time (seconds); `fast_url` + `fast_api_key` (any OpenAI-compatible endpoint, e.g.
`https://api.anthropic.com/v1` or a router) make them one HTTP request on a kept-alive connection.

```
jig ai                                 the UI with the chat open
jig ai why is parser failing?          one turn in the terminal; jig ai -c "…" continues it
jig resume                             pick a conversation or saved UI session: enter continues, d closes
jig ai config                          the setup; jig ai check tests it
jig ai config provider=deepseek api_key=sk-…
jig ai config provider=openai url=https://…/v1 api_key=… model=…
jig ai config proxy=localhost:10808    every AI request goes through it
jig ai config fast_url=https://…/v1 fast_api_key=… fast_model=…   completions as one direct request
```

**Local models.** `provider=ollama` (http://localhost:11434/v1) or `provider=lmstudio`
(http://localhost:1234/v1) run everything on this machine with no key and no data leaving it:
`ollama pull qwen2.5-coder`, then `jig ai config provider=ollama model=qwen2.5-coder` (`jig ai config`
lists what a running server serves). Completions go to it directly; the chat runs through opencode,
which needs to be installed. The tradeoffs: small models (7B and under) complete queries well but
pick tools badly, so the chat may answer without looking or call jig's tools with wrong arguments;
prefer a model trained for tool calling (qwen2.5-coder, llama3.1, mistral-nemo) and 14B or more for the
chat, and expect a turn to take longer than a hosted model on a laptop GPU.

No setup is needed: an opencode or Claude Code login is used as it is, and without one opencode's free
models are. Providers: `own`, `opencode`, `openai` (any compatible endpoint), `9router`, `deepseek`,
`anthropic` (for Claude Code). The key lives in `~/.config/jig/ai.json` (0600).

Secrets never reach the model: every prompt, completion and tool result goes through one redactor that
puts `<secret:NAME>` in place of `secrets:` values, `${NAME}`s with a secret's name taken from your
environment, env vars and component fields named like a password, token or key, passwords inside URLs
and DSNs, and whatever [goxang/scrub](https://github.com/goxang/scrub)'s packs catch in logs: JWTs, cloud
tokens, bearer headers, private keys, card numbers, PINs. `/redact` in the chat, or
`jig ai config redact=false|true`, turns it off and on.

Guard rails, enforced by jig's MCP server rather than the prompt: a conversation is bound to the
environment it started on (other `env`s are refused); the project directory is the only workspace, with
credentials, `ai.deny` paths and `.git` out of reach; files change only through `jig_file` (create, edit, move;
deleting asks you), never a shell. Reads run at once; every change (restart, deploy, a KV put, a write
query, a task, ...) first shows its command and waits for your yes: enter runs it, `a` runs it and
allows that same command for the rest of the session. With no terminal or UI to ask on, only a change
your message asked for in so many words runs. A read-only environment refuses changes whoever confirms.
Secrets and `jig mcp`/`debug` are out of reach.

`jig why <service or symptom>` (or `/why` in the chat) is incident mode: `jig why api`, or
`jig why checkout is slow since the deploy`, which looks at the service the words name (by name, image or
`SERVICE_NAME`) and at every service when they name none. jig gathers the evidence itself, the same way
every time: the state, restarts and health of the service and everything it depends on, whether the
components they host answer, error lines in their logs and which services those lines name, failed and
slow traces, the dashboards' metrics about them (where each started in the window and where it is now),
firing alerts, recent restarts and changes (deploys, the assistant's own). It ranks the suspects by that
evidence (down beats unreachable beats named in errors), then the assistant, with jig's tools to look
further, writes a summary, hypotheses citing the evidence IDs and a proposed fix. The conversation goes
on: answer `fix` and it applies the fix (each change waits for your yes) and checks the symptom is gone,
ask a follow-up, or press enter to leave; `jig ai -s <id>` picks it up later. `--fix` (or `/why --fix`)
applies the fix right after the analysis. `--no-ai` stops at the ranking, `--report` or `-o file` saves a
Markdown report (summary, timeline, evidence, next steps), `--json` hands it to tools.

Every action the assistant ran or tried is appended to `.jig/<env>/ai-audit.jsonl` (time, environment,
command, risk, result, who confirmed); `jig ai log` lists the changes and refusals (`--all` adds reads,
`--json` for tools), and the chat shows each one as it happens.

## GoLand

`o` on the Services screen (or `jig ide --open`) writes each service of the environment into GoLand's
Services view (`alt+8`), one folder per `sections:` entry plus `jig · infra`:

- `<service>` runs `jig attach`: starts it if it is down and shows its live log; stop stops it (infrastructure
  only stops following). Started or stopped in jig, GoLand follows: the config ends when the service does.
- `<service> · debug` (Go services): press Debug on it and GoLand starts the service if it is down, brings its
  debugger up in the background (`jig debug --detach --start`, on the service's stable port) and attaches;
  breakpoints work from there, on local processes, docker, kind and Kubernetes alike.
  `<service> · stop debugger` ends it (or `jig debug --stop <service>`).

They are jig commands on the same environment, so the TUI and GoLand show the same processes. GoLand
rewrites `.idea/workspace.xml` when it closes: if the Services view does not list them, add Shell Script
and Go Remote there (`+` › Run Configuration Type).

## Agents

```json
{ "mcpServers": { "jig": { "command": "jig", "args": ["mcp"], "cwd": "/path/to/project" } } }
```

Agents get terse output (`--brief`, `JIG_BRIEF=1`: tab-separated, no colour, long cells cut). Every
TUI screen has a CLI twin: `jig logs -E`, `jig metrics`, `jig profile`, `jig query --every 30s --times 10`,
`jig load`, `jig data`, `jig alerts`.

Tools: `jig_init` (draft or write jig.yaml for the project, with infrastructure presets), `jig_docs` (every
jig.yaml key; `search` returns only the sections holding some words), `jig_envs`, `jig_status`, `jig_up`, `jig_down`, `jig_service`, `jig_scale`, `jig_build`, `jig_deploy`,
`jig_logs`, `jig_query`, `jig_load`, `jig_kv`, `jig_infra`, `jig_task`, `jig_test`, `jig_ui` (acts in a jig UI that
started the agent: opens a screen straight at a row (`select`, Data's `path`, `filter`), shows only some
Metrics panels, presses keys, and runs several of these in one call (`steps`); each call answers with the
screen once loaded, so "open redis-atomic db 1" is one call; a change those keys start waits
for your confirmation, and `O` on Metrics shows every panel again), `jig_file` (list, read, write, edit, move, delete inside the project; credentials and `.git`
refused, delete needs `"confirm": true`), and `jig` for any other command.
Each runs the CLI, so protections apply: changes to a protected environment need `"confirm": true`.

Plugins are reached through meta-tools instead of one tool each: `jig_find` ranks actions, sources and
skills for a few words, `jig_call` runs an action, `jig_read` reads a source (wrapped as untrusted data),
`jig_open` resolves a ref and `jig_skill` loads a plugin's skill. `jig mcp --flat` also lists every plugin
action and source as its own tool. Writes by the assistant wait in `jig inbox`: it lists what each would
do, `jig inbox approve <id>` runs one as the assistant (audited with you as approver), `reject` drops it.

`jig skill --install` puts a skill into `.claude/skills/jig` and `.agents/skills/jig` (and each plugin's skills beside it), so Claude Code, Codex,
opencode and others know how to write and extend jig.yaml (services, infrastructure, `otel:`, `ui.tabs`,
dashboards, tests) and check it with `jig env`; `jig docs config` is the reference they read, and `jig docs <words>`
prints only the sections that hold them (`jig docs tasks steps`), across every doc.

## Server

`jig serve` shares a project with a team over HTTPS: remote CLIs (`jig -r <name>`), MCP clients and a
web inbox at `/` all go through the same Engine API, with policy enforced on the server.

```bash
jig admin users add alice --role dev --totp   # accounts live with the server; --totp prints the secret to scan
jig serve --addr :7443 --host jig.example.local --pull 1m
jig login https://jig.example.local:7443 --name work   # pins a self-signed certificate after you compare it
jig -r work call runtime.restart service=api  # runs there, as you
jig -r work -e loadtest2 lock --for 2h        # a lease: nobody else changes loadtest2 until you unlock
jig -r work inbox approve <id>                # four eyes: another person approves
jig -r work admin tokens create ci            # an API token for CI: jig login <url> --token <token>
jig -r work admin audit --who alice
```

Without `--cert`/`--key` the server makes a self-signed certificate once and prints its fingerprint.
Sign-in is a password (argon2id) with an optional TOTP code, OIDC (`--oidc-issuer`, `--oidc-client-id`,
`--oidc-redirect`; the secret in `$JIG_OIDC_CLIENT_SECRET`; `jig login --oidc` opens the browser), or an
API token. Built-in roles `viewer`, `dev`, `operator` (reauth on danger), `admin` and `ai` apply where the
project's `policy:` does not define them; an account with no known role only reads. A `reauth` rule
asks for the password or a TOTP code again (valid five minutes), `confirm` asks yes or no (`--yes`),
`approve` puts the write in the inbox for another person whose own role allows it. Agents and scheduled
reports run on the server for as long as it does. With `-r`, these commands work remotely: `call`,
`inbox`, `lock`, `unlock`, `whoami`, `mcp`, `admin tokens` and `admin audit`.
