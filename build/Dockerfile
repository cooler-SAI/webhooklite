FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
RUN go mod tidy
COPY . .
RUN go build -o webhook ./cmd/webhooklite

FROM alpine:latest
RUN apk add --no-cache ca-certificates
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app
COPY --from=builder /app/webhook .
COPY certs/ /certs/

RUN chown -R appuser:appgroup /app && \
    chown -R appuser:appgroup /certs && \
    chmod 644 /certs/*

USER appuser
EXPOSE 8443
CMD ["./webhook"]