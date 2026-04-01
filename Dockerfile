# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o drone-ci-mcp .

# Final stage — minimal image
FROM scratch

COPY --from=builder /app/drone-ci-mcp /drone-ci-mcp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080

ENTRYPOINT ["/drone-ci-mcp"]
