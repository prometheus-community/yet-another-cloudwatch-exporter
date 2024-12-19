ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest
LABEL maintainer="The Prometheus Authors <prometheus-developers@googlegroups.com>"

ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/yace /bin/yace

COPY examples/ec2.yml /etc/yace/config.yml

EXPOSE     5000
USER       nobody
ENTRYPOINT [ "/bin/yace" ]
CMD        [ "--config.file=/etc/yace/config.yml" ]
