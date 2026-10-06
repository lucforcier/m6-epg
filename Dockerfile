FROM golang:1.24 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /m6-epg ./cmd/m6-epg

FROM alpine:3.22

RUN addgroup -S m6epg && adduser -S -G m6epg m6epg
COPY --from=build /m6-epg /m6-epg

RUN mkdir -p /data && chown m6epg:m6epg /data

LABEL org.opencontainers.image.source="https://github.com/lucforcier/m6-epg"

USER m6epg
EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["/m6-epg"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
