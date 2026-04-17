FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /market-platform ./cmd/server \
 && CGO_ENABLED=0 go build -o /iecli ./cmd/iecli

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /market-platform /market-platform
COPY --from=builder /iecli /usr/local/bin/iecli
EXPOSE 8080 8081
CMD ["/market-platform"]
