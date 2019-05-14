# Qrator exporter
[![Build Status](https://travis-ci.org/StupidScience/qrator-exporter.svg?branch=master)](https://travis-ci.org/StupidScience/qrator-exporter)
[![Coverage Status](https://coveralls.io/repos/github/StupidScience/qrator-exporter/badge.svg)](https://coveralls.io/github/StupidScience/qrator-exporter)
[![Go Report Card](https://goreportcard.com/badge/github.com/StupidScience/qrator-exporter)](https://goreportcard.com/report/github.com/StupidScience/qrator-exporter)

Get statistics for all domains connected to account from [Qrator API](https://api.qrator.net/) and expose it in prometheus format.

## Configuration

Exporter configurates via environment variables:

|Env var|Description|
|---|---|
|QRATOR_CLIENT_ID|Your client ID for qrator. Only digits required.|
|QRATOR_X_QRATOR_AUTH|X-Qrator-Auth header for access to Qrator Api. It's not required if you use IP auth|

Exporter listen on tcp-port `9502`. Metrics available on `/metrics` path.

## Exposed metrics

It returns all statistics that defined [here](https://api.qrator.net/#types-statisticsresponse).
