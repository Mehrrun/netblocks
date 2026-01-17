# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the bot
RUN go build -o netblocks-telegram-bot ./cmd/telegram-bot

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/netblocks-telegram-bot .

# Run the bot
CMD ["./netblocks-telegram-bot"]

