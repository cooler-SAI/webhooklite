FROM golang:1.26-alpine AS builder

WORKDIR /app
# Copy both files for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copying all code for building the binary
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -o webhook ./cmd/webhooklite

FROM alpine:latest
RUN apk add --no-cache ca-certificates

# Create rootless user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app
COPY --from=builder /app/webhook .

# Set proper ownership
RUN chown -R appuser:appgroup /app

USER 10001:10001
EXPOSE 8443
CMD ["./webhook"]