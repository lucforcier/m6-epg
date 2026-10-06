FROM golang:1.24 AS build

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /m6-epg ./cmd/m6-epg

FROM gcr.io/distroless/static-debian12
COPY --from=build /m6-epg /m6-epg
ENTRYPOINT ["/m6-epg"]
