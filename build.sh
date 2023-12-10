#!/bin/sh

go build -ldflags="-s -w" -o xdp-pktgen ./cmd/pktgen/*.go