# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o kc-server ./cmd/server/
RUN CGO_ENABLED=0 GOOS=linux go build -o kc-community ./cmd/cli/

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/kc-server .
COPY --from=builder /app/kc-community .

RUN mkdir -p /app/data/assets

EXPOSE 8080

ENTRYPOINT ["./kc-server"]
