FROM golang:1.26.10-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/zeno-node ./cmd/zeno-node
RUN CGO_ENABLED=0 go build -o /out/zeno-controlplane ./cmd/zeno-controlplane

FROM alpine:3.21 AS node
WORKDIR /app
COPY --from=builder /out/zeno-node /usr/local/bin/zeno-node
EXPOSE 6380
ENTRYPOINT ["zeno-node"]

FROM alpine:3.21 AS controlplane
WORKDIR /app
COPY --from=builder /out/zeno-controlplane /usr/local/bin/zeno-controlplane
COPY zeno.yaml /app/zeno.yaml
EXPOSE 6379
ENTRYPOINT ["zeno-controlplane"]
