FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o zeno ./cmd/zeno

FROM alpine:3.21
RUN apk add --no-cache redis
WORKDIR /app
COPY --from=builder /app/zeno .
EXPOSE 6379 6380
CMD ["./zeno"]
