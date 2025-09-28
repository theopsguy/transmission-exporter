# Transmission Exporter for Prometheus

[![Test](https://github.com/theopsguy/transmission-exporter/actions/workflows/test.yml/badge.svg)](https://github.com/theopsguy/transmission-exporter/actions/workflows/test.yml)
[![Docker](https://github.com/theopsguy/transmission-exporter/actions/workflows/docker.yml/badge.svg)](https://github.com/theopsguy/transmission-exporter/actions/workflows/docker.yml)
[![Build Release](https://github.com/theopsguy/transmission-exporter/actions/workflows/build_release.yml/badge.svg)](https://github.com/theopsguy/transmission-exporter/actions/workflows/build_release.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/theopsguy/transmission-exporter.svg?maxAge=604800)](https://hub.docker.com/r/theopsguy/transmission-exporter)
[![Go Report Card](https://goreportcard.com/badge/github.com/theopsguy/transmission-exporter)](https://goreportcard.com/report/github.com/theopsguy/transmission-exporter)

Prometheus exporter for [Transmission](https://transmissionbt.com/) metrics, written in Go.

**This project is a fork of [metalmatze/transmission-exporter](https://github.com/metalmatze/transmission-exporter), which is no longer maintained.**
It is now actively maintained, and contributions are welcome.

## Running this exporter

### From binaries

Download a suitable binary from [the releases tab](https://github.com/theopsguy/transmission-exporter/releases).

Then:
```bash
./transmission-exporter <flags>
```

### Docker
```bash
docker pull theopsguy/transmission-exporter
docker run -d -p 19091:19091 metalmatze/transmission-exporter
```

### Kubernetes (Prometheus)

A sample kubernetes manifest is available in [example/kubernetes](https://github.com/theopsguy/transmission-exporter/blob/master/examples/kubernetes/transmission.yml)

Please run: `kubectl apply -f examples/kubernetes/transmission.yml`

You should:
* Attach the config and downloads volume
* Configure the password for the exporter

Your prometheus instance will start scraping the metrics automatically. (if configured with annotation based discovery). [more info](https://www.weave.works/docs/cloud/latest/tasks/monitor/configuration-k8s/)

### Docker Compose

Example `docker-compose.yml` with Transmission also running in docker.

```
transmission:
  image: linuxserver/transmission
  restart: always
  ports:
    - "127.0.0.1:9091:9091"
    - "51413:51413"
    - "51413:51413/udp"
transmission-exporter:
  image: theopsguy/transmission-exporter
  restart: always
  links:
    - transmission
  ports:
    - "127.0.0.1:19091:19091"
  environment:
    TRANSMISSION_ADDR: http://transmission:9091
```

## Building the Exporter

### Local Build

This project uses [promu](https://github.com/prometheus/promu) to build the binary.

To build locally, run:

```bash
make build
```

### Building with Docker
After a successful local build:
```
docker build -t transmission-exporter .
```

## Configuration

ENV Variable | Description
|----------|-----|
| WEB_PATH | Path for metrics, default: `/metrics` |
| WEB_ADDR | Address for this exporter to run, default: `:19091` |
| WEB_CONFIG_FILE | Configuration file to protect exporter with TLS and/or basic auth, no default |
| TRANSMISSION_ADDR | Transmission address to connect with, default: `http://localhost:9091` |
| TRANSMISSION_USERNAME | Transmission username, no default |
| TRANSMISSION_PASSWORD | Transmission password, no default |
| LOG_FORMAT | Specify log output format. Options are 'text' (default) or 'json' |

## TLS and Basic Authentication

This exporter uses the Prometheus exporter-toolkit to provide TLS and basic authentication support.

See the [exporter-toolkit web configuration](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md) for more details.

### Original authors of the Transmission package  
Tobias Blom (https://github.com/tubbebubbe/transmission)  
Long Nguyen (https://github.com/longnguyen11288/go-transmission)
