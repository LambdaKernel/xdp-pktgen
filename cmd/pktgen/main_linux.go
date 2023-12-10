//go:build linux

package main

import (
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
)

func getRoutes(destination net.IP) ([]netlink.Route, error) {
	routes, err := netlink.RouteGet(destination)
	if err != nil {
		return nil, err
	}

	return routes, nil
}

func getNeighs(gw netlink.Route) ([]netlink.Neigh, error) {
	neighs, err := netlink.NeighList(gw.LinkIndex, syscall.AF_INET)
	if err != nil {
		panic(err)
	}

	return neighs, nil
}

func linkByIndex(gw netlink.Route) (netlink.Link, error) {
	link, err := netlink.LinkByIndex(gw.LinkIndex)
	if err != nil {
		return nil, err
	}

	return link, nil
}
