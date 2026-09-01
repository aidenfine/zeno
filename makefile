APP_EXECUTABLE=zeno-build

.PHONY: proto build run clean bench loadtest

proto:
	rm -f pb/*.pb.go
	protoc --go_out=. --go_opt=module=zeno --go-grpc_out=. --go-grpc_opt=module=zeno proto/*.proto

build:
	GOARCH=amd64 GOOS=darwin go build -o ${APP_EXECUTABLE}-darwin ./cmd/zeno
	GOARCH=amd64 GOOS=linux go build -o ${APP_EXECUTABLE}-linux ./cmd/zeno
	GOARCH=amd64 GOOS=windows go build -o ${APP_EXECUTABLE}-windows ./cmd/zeno

run: build
	./${APP_EXECUTABLE}-darwin

# Component micro-benchmarks (no server needed): RESP parse/marshal, the
# command handlers, and the queue. Runs anywhere.
bench:
	go test -run '^$$' -bench . -benchmem ./...

# End-to-end load test against a running leader (see bench/main.go docs).
# Pass flags through ARGS, e.g. `make loadtest ARGS="-cmd GET -clients 100"`.
loadtest:
	go run ./bench $(ARGS)

clean:
	go clean
	rm -f ${APP_EXECUTABLE}-darwin
	rm -f ${APP_EXECUTABLE}-linux
	rm -f ${APP_EXECUTABLE}-windows
