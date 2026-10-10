# Examples

Each folder is a whole project: a `jig.yaml` and the code it runs. Start with `hello`.

| example | runs on | what it shows |
|---|---|---|
| [hello](hello) | local processes | the smallest `jig.yaml`: one Go service, logs, metrics without Prometheus, a load generator, a test suite; opens in the simple view |
| [shop](shop) | docker (or local + docker infra) | an api and a worker over Postgres, Redis, RabbitMQ and Zipkin; `compose.yaml` is the same stack, and what `jig init` started from. Every screen has something real: orders in the database, the queue and its failures, one trace from api to worker, saved queries, alerts |
| [python](python) | local processes | a Python service (stdlib only): no build step, JSON logs, `/metrics` read by jig, load, unittest |
| [kind](kind) | kind, docker, local | Kubernetes: manifests in `deploy/`, an HPA, pprof, load from jig |

```bash
cd examples/hello
jig up        # build and start
jig           # the control plane (V switches simple / detailed view, ? lists the keys)
jig down
```

`examples/internal/demo` is what the services share (JSON logs, metrics, spans), so each `main.go`
is only about its own work.
