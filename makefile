.PHONY: proto build run run-node clean bench loadtest

proto:
	protoc --go_out=. --go_opt=module=zeno --go-grpc_out=. --go-grpc_opt=module=zeno proto/*.proto

build:
	go build -o bin/zeno-controlplane ./cmd/zeno-controlplane
	go build -o bin/zeno-node ./cmd/zeno-node

run: build
	./bin/zeno-controlplane $(ARGS)

run-node: build
	./bin/zeno-node $(ARGS)

bench:
	go test -run '^$$' -bench . -benchmem ./...

loadtest:
	go run ./bench $(ARGS)

clean:
	go clean
	rm -f bin/zeno-controlplane bin/zeno-node
