FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/idata-server ./cmd/idata-server

FROM alpine:3.22

RUN addgroup -S idata && adduser -S -G idata -h /var/lib/idata idata \
    && install -d -o idata -g idata -m 0700 /var/lib/idata
COPY --from=build /out/idata-server /usr/local/bin/idata-server

USER idata
EXPOSE 12345
VOLUME ["/var/lib/idata"]
ENTRYPOINT ["/usr/local/bin/idata-server"]
