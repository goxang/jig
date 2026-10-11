# Security

## Workspace trust

A project's own plugins (`jig.lock`, `.jig/plugins`, `plugins.<name>.path`) stay off until the
workspace is trusted. The first `jig` in a project lists what it would install, with permissions and
sources, and asks once; `jig trust` does the same. CI and agents pass `--yes` or set
`JIG_TRUST=project`; without trust a non-interactive run fails and names the command to run. Plugins
in `~/.config/jig/plugins` are yours and need no trust.

## Supply chain

- `jig.lock` pins each plugin's version, source, sha256 per platform and a hash of its permissions.
  Auto-install never goes past the lock.
- An update that adds permissions asks again.
- Official archives are signed with minisign; the public key is built into jig. A registry entry
  without a valid signature for that key does not install.

## Plugin processes

- A plugin gets `PATH`, `HOME`, `LANG`, `LC_ALL`, `TZ`, proxy variables and the variables its
  `permissions.env` lists; nothing else of jig's environment.
- Secrets reach it only through `host/secret`, and only those in `permissions.secrets`. It sees only
  its own config.
- On Linux plugins run under Landlock (`jig __sandbox`): the filesystem is limited to the project,
  the plugin's folder, temp directories and `permissions.fs`; on Landlock ABI 4 or later, TCP connects
  are limited to the ports `permissions.net` lists. Elsewhere permissions are advisory and audited, and
  `jig plugin info` says so.

## Secrets

`jig secret set NAME` keeps values in a 0600 store under `~/.config/jig`. Values are masked in every
view, in audit arguments and in what the AI sees.

## Policy

Every Engine call, from any client, goes through `policy:` (see [config](config.md#policy)). Rules are
`action[:effect][@env/<name>]`; the most restrictive match wins: deny, approve (another person, via
`jig inbox`), reauth (password or TOTP again), confirm, then allow. Without `policy:`, reads run, a
`readonly` environment refuses writes, a `protected` one and danger actions ask first, and the
assistant's writes wait in the inbox. Exit code 4 means protected or denied.

## Server

`jig serve` serves projects over TLS to the remote TUI and CLI (`jig login`, `jig -r <name>`).

- Accounts: local users with argon2id passwords and optional TOTP, OIDC, and scoped, expiring tokens
  (`jig admin`).
- Secrets stay on the server; clients see masked values.
- `jig lock env <name>` leases an environment so two people do not deploy over each other.

## Audit

Each environment appends to `.jig/<env>/audit.jsonl` (the server keeps its own): time, subject (user,
AI, agent or token), action, arguments with secrets masked, effect, decision, approver and result.
`jig admin audit` filters it.
