# Stage 1: Build
FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main .

# Stage 2: Slim runtime
FROM ubuntu:22.04

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 8080
CMD ["./main"]
