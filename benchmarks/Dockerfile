# Multi-stage Dockerfile for reproducible benchmarking
FROM golang:1.27.4-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bench-server benchmarks/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /bench-server .

EXPOSE 4001
ENV PORT=4001
CMD ["./bench-server"]
