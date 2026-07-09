FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/chat-server ./cmd/service/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/chat-server .
COPY --from=builder /app/web ./web
COPY --from=builder /app/config.yaml .

EXPOSE 8080

CMD ["./chat-server"]