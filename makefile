APP_EXECUTABLE=zeno-build

build:
	GOARCH=amd64 GOOS=darwin go build -o ${APP_EXECUTABLE}-darwin main.go
	GOARCH=amd64 GOOS=linux go build -o ${APP_EXECUTABLE}-linux main.go
	GOARCH=amd64 GOOS=windows go build -o ${APP_EXECUTABLE}-windows main.go

run: build
	./${APP_EXECUTABLE}-darwin

clean:
	go clean
	rm -f ${APP_EXECUTABLE}-darwin
	rm -f ${APP_EXECUTABLE}-linux
	rm -f ${APP_EXECUTABLE}-windows
