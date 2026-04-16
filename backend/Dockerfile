FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /openfakegps ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=builder /openfakegps /openfakegps

EXPOSE 8080 50051

ENTRYPOINT ["/openfakegps"]
