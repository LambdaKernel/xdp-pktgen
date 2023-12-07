package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"xdp-dos/netinfo"

	"github.com/asavie/xdp"
	"github.com/jessevdk/go-flags"
	"github.com/kr/pretty"
	"github.com/vishvananda/netlink"
)

var fopts struct {
	Verbose    bool   `short:"v" long:"verbose" description:"Verbose info to stdout"`
	Port       uint16 `short:"p" long:"port" description:"Destination host port" required:"true"`
	Randbuffer bool   `short:"r" long:"random-buffer" description:"Use random data for the buffer"`
	ipVersion  netinfo.IpVersion
}

const queueID = 0

func main() {

	args, err := flags.Parse(&fopts)
	if err != nil {
		panic("")
	}

	if len(args) != 1 {
		panic("missing argument")
	}

	raddr, err := net.ResolveIPAddr("ip", args[0])
	if err != nil {
		panic(err)
	}

	if raddr.IP.To4() == nil {
		fopts.ipVersion = netinfo.IPv6
		log.Println("Using IPv6")
	} else {
		fopts.ipVersion = netinfo.IPv4
		log.Println("Using IPv4")
	}

	host := raddr.IP
	port := fopts.Port

	routes, err := netlink.RouteGet(host)
	if err != nil {
		panic(err)
	}

	if len(routes) == 0 {
		panic("No routes to host")
	}

	gw := routes[0]

	neighs, err := netlink.NeighList(gw.LinkIndex, syscall.AF_INET)
	if err != nil {
		panic(err)
	}

	link, err := netlink.LinkByIndex(gw.LinkIndex)
	if err != nil {
		panic(err)
	}

	var dstMac net.HardwareAddr

	if gw.Gw == nil {
		dstMac, err = getNeighMac(neighs, gw.Dst.IP)
		if err != nil {
			panic(err)
		}
	} else {
		dstMac, err = getNeighMac(neighs, gw.Gw)
		if err != nil {
			panic(err)
		}
	}

	srcPort, err := netinfo.GetFreeUDPPort()
	if err != nil {
		panic(err)
	}

	srcUDP, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", gw.Src, srcPort))
	if err != nil {
		panic(err)
	}

	dstUDP, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host.String(), port))
	if err != nil {
		panic(err)
	}

	srcMac := link.Attrs().HardwareAddr

	fmt.Printf("sending UDP packets from %v (%v) to %v (%v)...\n", srcUDP.IP, srcMac, dstUDP.IP, dstMac)

	ctx, _ := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)

	xsk, err := xdp.NewSocket(link.Attrs().Index, queueID, nil)
	if err != nil {
		panic(err)
	}

	defer xsk.Close()

	magic := []byte{0x00, 0xff, 0xff, 0x00, 0xfe, 0xfe,
		0xfe, 0xfe, 0xfd, 0xfd, 0xfd, 0xfd, 0x12, 0x34,
		0x56, 0x78}

	timeNow := time.Now().UnixMilli()

	payload := make([]byte, 1492)
	payload[0] = 0x1
	payload[1] = byte(timeNow >> 56)
	payload[2] = byte((uint64(timeNow) & 0x00FFFFFFFFFFFFFF) >> 48)
	payload[3] = byte((uint64(timeNow) & 0x0000FFFFFFFFFFFF) >> 40)
	payload[4] = byte((uint64(timeNow) & 0x000000FFFFFFFFFF) >> 32)
	payload[5] = byte((uint64(timeNow) & 0x00000000FFFFFFFF) >> 24)
	payload[6] = byte((uint64(timeNow) & 0x0000000000FFFFFF) >> 16)
	payload[7] = byte((uint64(timeNow) & 0x000000000000FFFF) >> 8)
	payload[8] = byte(uint64(timeNow) & 0x00000000000000FF)

	payload = append(payload, magic...)

	if fopts.Randbuffer {
		_, err = rand.Read(payload)
		if err != nil {
			panic(err)
		}
	}

	buf, err := netinfo.NewUDPPacket(fopts.ipVersion, srcMac, dstMac, *srcUDP, *dstUDP, payload)
	if err != nil {
		panic(err)
	}

	frameLen := len(buf)

	if fopts.Verbose {

		go func(xsk *xdp.Socket) {
			var err error
			var prev xdp.Stats
			var cur xdp.Stats
			var numPkts uint64
			for i := uint64(0); ; i++ {
				time.Sleep(time.Duration(1) * time.Second)
				cur, err = xsk.Stats()
				if err != nil {
					panic(err)
				}
				numPkts = cur.Completed - prev.Completed
				fmt.Printf("%d packets/s (%d Mb/s)\n", numPkts, (numPkts*uint64(frameLen)*8)/(1000*1000))
				prev = cur
			}
		}(xsk)
	}

	for ctx.Err() == nil {

		descs := xsk.GetDescs(xsk.NumFreeTxSlots())
		for _, desc := range descs {
			copy(xsk.GetFrame(desc), buf)
		}

		xsk.Transmit(descs)
		xsk.Complete(len(descs))
	}

	<-ctx.Done()
}

func getNeighMac(neighs []netlink.Neigh, ip net.IP) (net.HardwareAddr, error) {

	for _, neigh := range neighs {
		if neigh.IP.Equal(ip) {
			return neigh.HardwareAddr, nil
		}
	}

	_, err := pretty.Println(neighs)
	if err != nil {
		return nil, err
	}

	_, err = pretty.Println(ip)
	if err != nil {
		return nil, err
	}

	return nil, errors.New("neigh IP not found in ARP table")
}
