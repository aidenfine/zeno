# Zeno

A key value store with separate control plane and storage node runners.

## Run locally

Build both binaries with Go 1.26.10:

```sh
make build
```

Run each command in its own terminal:

```sh
./bin/zeno-node --name node-a --listen 127.0.0.1:6380
./bin/zeno-node --name node-b --listen 127.0.0.1:6381
./bin/zeno-controlplane --config examples/local.yaml
```

Connect clients to the control plane:

```sh
redis-cli -p 6379 SET greeting hello
redis-cli -p 6379 GET greeting
```

`make run` starts the control plane; `make run-node` starts a node. Both accept
flags through `ARGS`. The old combined `cmd/zeno` entry point has been removed.

## Deploy

Build distinct images from the same repository:

```sh
docker build --target node -t zeno-node .
docker build --target controlplane -t zeno-controlplane .
```

The final node image contains only the node binary, without the source tree or
control plane binary. Deploy one node container per storage machine and one
control plane container with your cluster configuration mounted:

```sh
docker run --rm -p 6380:6380 zeno-node
docker run --rm -p 6379:6379 \
  -v "$PWD/zeno.yaml:/app/zeno.yaml:ro" zeno-controlplane
```

## Logging

Server logs use [flog](https://github.com/aidenfine/flog), with timestamps and
named fields written to stderr. The shared configuration lives in
`src/logger/logger.go` and defaults to INFO. Set its `LogLevel` to `DEBUG` to
include command diagnostics. Since flog has no WARN level, warnings are emitted
at INFO with a `severity=WARN` field.

Use `--name node-b` to label all logs from a node, including heartbeats and
commands: `[INFO] [node-b] - received heartbeat ...`. The default node name is
the machine hostname (or container hostname). The control plane defaults to
`zeno-controlplane` and also accepts `--name`. This label is for logging; cluster
membership names remain configured separately in YAML.
