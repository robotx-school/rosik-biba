FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/bibika-server \
    .

FROM alpine:3.23

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && addgroup -S bibika \
    && adduser -S bibika -G bibika

WORKDIR /app
COPY --from=builder /out/bibika-server /app/bibika-server

USER bibika
EXPOSE 31415/tcp
EXPOSE 31415/udp
ENTRYPOINT ["/app/bibika-server"]
