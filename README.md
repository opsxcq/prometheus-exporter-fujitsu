# Prometheus exporter for Fujitsu RX300 hardware
![License](https://img.shields.io/badge/License-GPL-blue.svg?style=plastic)

[![Docker Pulls](https://img.shields.io/docker/pulls/strm/prometheus-exporter-fujitsu.svg?style=plastic)](https://hub.docker.com/r/strm/prometheus-exporter-fujitsu/)

Exporter that makes possible scrap data from Fujitsu RX300 servers. Collects
data regarding power usage and temperature.

# Configuration

This exporter is intended to run as a docker container using the following
environment variables:

 - `FUJITSU_URL`: URL for your hardware
 - `FUJITSU_USER`: User to log in into the web interface
 - `FUJITSU_PASS`: The password for the respective user
 
# Running

If using ansible start the container with:

```
  - name: "Prometheus | Exporter Fujitsu"
    docker_container:
      name: prometheus-exporter-fujitsu
      image: strm/prometheus-exporter-fujitsu
      state: started
      restart_policy: unless-stopped
      memory: '256m'
      env:
        FUJITSU_URL: 'http://your.hardware'
        FUJITSU_USER: 'admin'
        FUJITSU_PASS: 'XXX'
      networks:
        - name: monitoring
```
 
# Configuring prometheus

This exporter binds the port **9900**, configure it on `prometheus.yml` as the example
bellow:

```
  - job_name: 'fujitsu'
    static_configs:
      - targets: ['prometheus-exporter-fujitsu:9900']
```

