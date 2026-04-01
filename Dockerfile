# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o drone-ci-mcp .

# Final stage — minimal image with wget for healthchecks
FROM alpine:3

RUN apk add --no-cache wget ca-certificates

COPY --from=builder /app/drone-ci-mcp /drone-ci-mcp

EXPOSE 8080

ENTRYPOINT ["/drone-ci-mcp"]
