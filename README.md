Prometheus-PuppetDB
===================

[![Docker Pulls](https://img.shields.io/docker/pulls/camptocamp/prometheus-puppetdb.svg)](https://hub.docker.com/r/camptocamp/prometheus-puppetdb/)
[![Build Status](https://img.shields.io/travis/camptocamp/prometheus-puppetdb/master.svg)](https://travis-ci.org/camptocamp/prometheus-puppetdb)
[![Coverage Status](https://img.shields.io/coveralls/camptocamp/prometheus-puppetdb.svg)](https://coveralls.io/r/camptocamp/prometheus-puppetdb?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/camptocamp/prometheus-puppetdb)](https://goreportcard.com/report/github.com/camptocamp/prometheus-puppetdb)
[![By Camptocamp](https://img.shields.io/badge/by-camptocamp-fb7047.svg)](http://www.camptocamp.com)


Prometheus scape lists based on PuppetDB.


## Installing

```shell
$ go get github.com/camptocamp/prometheus-puppetdb
```

## Usage

```shell
Usage:
  prometheus-puppetdb [OPTIONS]

Application Options:
  -V, --version         Display version.
  -u, --puppetdb-url=   PuppetDB base URL. (default: http://puppetdb:8080) [$PROMETHEUS_PUPPETDB_URL]
  -q, --puppetdb-query= PuppetDB query. (default: facts { name='ipaddress' and nodes { deactivated is null and facts { name='collectd_version' and value ~ '^5\\.7' } and resources {
                        type='Class' and title='Collectd' } } }) [$PROMETHEUS_PUPPETDB_QUERY]
  -p, --collectd-port=  Collectd port. (default: 9103) [$PROMETHEUS_PUPPETDB_COLLECTD_PORT]
  -c, --config-file=    Prometheus target file. (default: /etc/prometheus-config/prometheus-targets.yml) [$PROMETHEUS_PUPPETDB_FILE]
  -s, --sleep=          Sleep time between queries. (default: 5s) [$PROMETHEUS_PUPPETDB_SLEEP]
  -m, --manpage         Output manpage.

Help Options:
  -h, --help            Show this help message
```
