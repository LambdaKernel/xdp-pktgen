#!/bin/sh

env CGO_LDFLAGS="-L ./lib -lbpf -lxdp" go build -ldflags="-s -w" -o xdp-pktgen ./cmd/pktgen/*.go