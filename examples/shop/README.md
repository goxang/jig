# shop

An api takes orders (`POST /orders?item=x`), stores them in Postgres and queues them on RabbitMQ;
a worker ships them and marks them done. One order in twenty fails and lands in `orders.failed`.

```bash
jig up --build       # Postgres, Redis, RabbitMQ, Zipkin, then api and worker
jig load run orders  # 20 orders/s (or the Load screen)
jig                  # watch them
jig down postgres redis rabbitmq zipkin api worker
```

Where to look in `jig`:

- **Data** › queue: `orders` and `orders.failed`; enter shows a queue's settings, `m` peeks its
  messages, enter on one opens its body field by field; space marks, `P` purges, `]` exchanges,
  bindings, connections
- **Data** › db: the `orders` table; **Queries**: orders per status, refreshed every 5s
- **Traces**: one trace per order, api › publish › worker; enter walks the spans and their tags
- **Logs**: pick api and worker (`f`), `v` opens a line field by field, `h` picks the fields shown

`docker compose up --build` runs the same stack without jig. `jig init` in a copy of this folder
without `jig.yaml` writes a first one from `compose.yaml`.
