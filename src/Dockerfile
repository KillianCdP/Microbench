# Build stage
FROM docker.io/library/golang:1.23.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o microbench ./cmd/service

FROM docker.io/library/alpine:3.20.3 AS final

RUN addgroup -g 3000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser

RUN apk --no-cache add ca-certificates tzdata && \
    rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=builder --chown=appuser:appgroup /app/microbench .

USER appuser

EXPOSE 50051 8080 6060

ENTRYPOINT ["/app/microbench"]
