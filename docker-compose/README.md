## Setting up a local docker-compose environment

This folder contains a [docker-compose](./docker-compose.yaml) configuration file to start a local development environment. 
This includes:
- YACE, using as config file [yace-config.yaml](./yace-config.yaml)
- Prometheus, with a scraping configuration targeting YACE
- Grafana, with no login required and the Prometheus datasource configured

Docker will mount the `~/.aws` directory in order to re-utilize the host's AWS credentials. For selecting which region
and AWS profile to use, fill in the `AWS_REGION` and `AWS_PROFILE` variables passed to the `docker-compose up` command,
as shown below.

```bash
# Build the YACE docker image
docker-compose build

# Start all docker-compose resource
AWS_REGION=us-east-1 AWS_PROFILE=sandbox docker-compose up -d 
```

After that, Prometheus will be exposed at [http://localhost:9090](http://localhost:9090), and Grafana in [http://localhost:3000](http://localhost:3000).
