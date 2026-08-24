# Build stage
FROM golang:1.27.0-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /copylingo ./cmd/server

# Runtime stage
FROM alpine:3.19

# ffmpeg: transcodes Gemini TTS raw PCM into Telegram-ready OGG/Opus (ADR-031).
RUN apk add --no-cache ca-certificates tzdata ffmpeg

WORKDIR /app
COPY --from=builder /copylingo .

EXPOSE 8080

CMD ["./copylingo"]
