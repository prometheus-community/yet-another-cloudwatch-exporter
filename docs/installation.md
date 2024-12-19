# Installing and running YACE

There are various way to run YACE.

## Binaries

See the [Releases](https://github.com/prometheus-community/yet-another-cloudwatch-exporter/releases) page to download binaries for various OS and arch.

## Docker

Docker images are available on GitHub Container Registry [here](https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pkgs/container/yet-another-cloudwatch-exporter).

The image name is `ghcr.io/nerdswords/yet-another-cloudwatch-exporter` and we only support tags of the form `vX.Y.Z`.

To pull and run the image locally use:

```shell
docker run -d --rm \
  -v $PWD/credentials:/exporter/.aws/credentials \
  -v $PWD/config.yml:/tmp/config.yml \
  -p 5000:5000 \
  --name yace ghcr.io/nerdswords/yet-another-cloudwatch-exporter:vX.Y.Z
```

Do not forget the `v` prefix in the image version tag.

## Docker compose

See the [docker-compose directory](../docker-compose/README.md).

## Kubernetes

### Install with HELM

The official [HELM chart](https://github.com/nerdswords/helm-charts) is the recommended way to install YACE in a Kubernetes cluster.

### Install with manifests

Example:

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: yace
data:
  config.yml: |-
    ---
    # Start of config file
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: yace
spec:
  replicas: 1
  selector:
    matchLabels:
      name: yace
  template:
    metadata:
      labels:
        name: yace
    spec:
      containers:
      - name: yace
        image: ghcr.io/nerdswords/yet-another-cloudwatch-exporter:vX.Y.Z # release version as tag - Do not forget the version 'v'
        imagePullPolicy: IfNotPresent
        args:
          - "--config.file=/tmp/config.yml"
        ports:
        - name: app
          containerPort: 5000
        volumeMounts:
        - name: config-volume
          mountPath: /tmp
      volumes:
      - name: config-volume
        configMap:
          name: yace
```
