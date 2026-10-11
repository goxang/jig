# {%.Name%}

A [jig](https://github.com/goxang/jig) plugin.

```bash
jig plugin dev        # link it into a project, rebuild on change, show the RPC trace
jig plugin test       # check the manifest, its sources and contracts, replay testdata/*.jsonl
jig plugin add ./     # or install it from a path, a git URL or a release archive
```

Release: push a tag such as `v0.1.0`; the workflow attaches the archive to a GitHub release and
prints the registry index entry to propose in [goxang/jig-plugins](https://github.com/goxang/jig-plugins).
