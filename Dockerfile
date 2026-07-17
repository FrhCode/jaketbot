FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go test ./... && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /website-watcher .

FROM alpine:3.22
RUN adduser -D -H -u 10001 appuser && mkdir -p /data && chown appuser:appuser /data
COPY --from=build /website-watcher /usr/local/bin/website-watcher
USER appuser
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/website-watcher"]
