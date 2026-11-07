FROM golang:1.27.0-alpine AS builder

WORKDIR /src
COPY . .

RUN apk add --no-cache git make && \
    go mod download && \
    make build OUTPUT=harp

FROM alpine:3.24.2

WORKDIR /data

COPY --from=builder "/src/harp" "/usr/local/bin/harp"

ENTRYPOINT ["harp"]
