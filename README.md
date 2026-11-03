# Zeno

A key value store with separate control plane and storage node runners.

## Run locally

Build both binaries with Go 1.26.10:

```sh
make build
```

Run each command in its own terminal:

```sh
./bin/zeno-node --listen 127.0.0.1:6380
./bin/zeno-node --listen 127.0.0.1:6381
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
