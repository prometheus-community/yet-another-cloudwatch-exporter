FROM golang:1.23-alpine AS builder
ARG VERSION

RUN apk update && apk add --no-cache ca-certificates git make tzdata && update-ca-certificates

ENV USER=exporter
ENV UID=10001

WORKDIR /opt/

RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

COPY . ./

RUN go mod download && GOOS=linux CGO_ENABLED=0 go build -v -ldflags '-X main.version=$VERSION -w -s -extldflags "-static"' -a -o yace ./cmd/yace

FROM scratch

# copy from builder docker container
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

WORKDIR /exporter/

COPY --from=builder /opt/yace /usr/local/bin/yace
USER exporter:exporter

EXPOSE 5000
CMD ["--config.file=/tmp/config.yml"]
ENTRYPOINT ["yace"]
