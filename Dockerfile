# Multi-stage build for Go binary
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/greek-tv-scraper ./cmd/server

# Production image
FROM alpine:3.21 AS server
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
RUN apk add --no-cache tzdata
COPY --from=builder /bin/greek-tv-scraper /usr/local/bin/greek-tv-scraper
EXPOSE 8082
ENTRYPOINT ["greek-tv-scraper"]
