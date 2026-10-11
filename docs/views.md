# Views

A view is a screen built from widgets bound to sources. Views live under `views:` in jig.yaml, in a
plugin's manifest, or in their own file (`file: views/board.yaml`).

```yaml
views:
  Board:
    key: B                       # opens it from anywhere
    grid:
      - [{widget: stat, title: Orders/min, source: metrics, query: 'sum(rate(orders_total[1m]))'},
         {widget: table, title: Slow queries, source: db, query: 'select ...', refresh: 30s}]
      - [{widget: terminal, title: Shell}]
```

## Slots

| slot | where it shows |
|---|---|
| `tab` | a top-level screen (the default) |
| `home` | a tile on the Home screen, which appears once any view has this slot |
| `{tab: {key: G}}` | a screen with its own key |
| `off` | nowhere; a plugin uses it to keep its contract's default view away |

`modes: [software, product]` shows a view only in those UI modes (`V`); a view without `modes:` shows in
every mode but `product`.

A plugin installed for the user (`~/.config/jig/plugins`) shows its views in a project only when
jig.yaml configures it under `plugins:`, its `detect:` matches (`files` globs, `git_remote` in
`.git/config`), or it has neither `detect:` nor a config schema. A contract's default view shows only
for a plugin without views of its own. Screens come first, then the views of plugins serving vcs,
scm, tracker and docs, then the project's own, then the ones a user installed for themselves.

## Layouts

`grid: [[a, b], [c]]` (rows of cells) · `tabs: {Name: widget, ...}` (sub-tabs, ctrl+←→) ·
`split: [a, b]` · `stack: [a, b]` · `box: widget` · `scroll: widget`. A child takes `height:` or
`width:` (rows, columns or a percentage) and `span:` (a weight). `border: false` drops a panel's frame.

## Bindings

`source:` is a source name, a component name (`db`, `metrics`, `runtime`) or `plugins`. With it:
`query:`, `args:`, `refresh:`/`every:` (default 10s), `stream: true` (appends batches, `keep:` rows,
default 5000). Only visible widgets fetch; equal bindings share one request and its cache; errors back
off up to a minute and keep the last frame. `r` refetches.

`transform:` is a list of steps run on the frame:
`filter: "status!=ok"` (`=` `!=` `~` `>` `<` `>=` `<=`, or a bare word) · `sort: -latency` · `limit: 20` ·
`select: [a, b]` · `rename: {a: b}` · `group: {by: svc, agg: {n: count, p: avg(latency)}}` ·
`join: {source: x, query: ..., on: id}` (left join).

## Widgets

| widget | props |
|---|---|
| `table` | `columns: [{name, color, ref, width, hide}]`, `actions`; enter opens a row's ref, `y`/`Y` copy, `/` filters |
| `list` `kv` `tree` | the frame as lines, key/value pairs, a JSON tree |
| `stat` `gauge` `progress` | `value`, `unit`, `min`, `max`, `thresholds`, `color` |
| `bar` `sparkline` `chart` `heatmap` | `series`, `time`, `value`, `unit`, `colors` |
| `text` `markdown` `code` `diff` | `text`, `file`, `field` (one column of the frame), or the frame |
| `logs` | a stream of lines; `stream: true` |
| `waterfall` `timeline` `graph` | spans, events, nodes and edges |
| `calendar` | `time`; the month, with a count on each day that has events |
| `board` | `by` (the column field), `card` (the title field), `key` (shown before it), `columns`; arrows or hjkl move, `/` filters |
| `form` | `schema`, or `action` (its args), `submit` |
| `terminal` | `command` (string or list; default `$SHELL`), `dir` (default the project; relative to the view's own folder), `env`, `keep` (runs while hidden). Keys, and clicks when the program asks for the mouse, go to the program; ctrl+] leaves |
| `canvas` | drawn by `plugin` through `widget.render` |

`widget: calendar@2` asks for a version; a jig without it, or without the widget, renders `fallback:`
(another widget) instead. Plugins get the host's widget versions at `initialize`.

Colours are theme tokens: `ok`, `warn`, `err`, `info`, `accent`, `muted`, `border`, `fg`, `series1`…

## Actions

```yaml
actions:
  - {key: R, name: retry, action: queue.retry, args: {id: '${row.id}'}}
  - {key: L, name: logs, command: 'logs ${row.service}'}
  - {key: D, name: drop, command: 'db drop ${row.name}', danger: true}
```

`action:` runs through the engine (policy, confirmation, audit). `command:` is a palette line, the CLI's
grammar (`open Jira ${row.issue}` jumps to a view, filtered). `ai:` opens the chat with the prompt typed
in. In args, `${ask}` asks first, `${ask:field}` asks with the row's value filled in, `${pick:field}`
picks from the values the frame has; `danger: true` confirms.

A table or board takes `preview:` (a widget, `width: 45%`, `hidden: true`) shown beside it for the
selected row: `${row.x}` in its args and title. `v` shows or hides it, enter on a board card too. A
binding with a `query` arg gets `Q` to edit it.

## Keys

The `jig` preset: `:` palette (`:restart parser`, `:kv get key`, `:open Logs`), `ctrl+k` search,
`g l|m|t|d|s|g` go to, `.` or right-click a menu of the screen's keys, `ctrl+e` environment, `T` tasks.
`keys: rig` in `.jig/local.yaml` or `~/.config/jig/config.yaml` restores rig's keys (`E`, `N`, `S`,
`ctrl+k` kubeconfig, `ctrl+e` environment details).
