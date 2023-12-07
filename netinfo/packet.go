package netinfo

import (
	"net"
	"net/netip"
	"syscall"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type IpVersion int

const (
	IPv4 IpVersion = syscall.AF_INET
	IPv6 IpVersion = syscall.AF_INET6
)

func NewUDPPacket(ipVersion IpVersion, srcMac, dstMac net.HardwareAddr, srcAddr, dstAddr net.UDPAddr, payload []byte) ([]byte, error) {
	srcIP := srcAddr.IP
	dstIP := dstAddr.IP
	srcPort := srcAddr.Port
	dstPort := dstAddr.Port

	// fmt.Printf("SRC_IP: %s\nDST_IP: %s\nSRC_PORT: %d\nDST_PORT: %d\n", srcIP.String(), dstIP.String(), srcPort, dstPort)

	var eth *layers.Ethernet
	var ip4 *layers.IPv4
	var ip6 *layers.IPv6

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	udp := &layers.UDP{
		SrcPort: layers.UDPPort(srcPort),
		DstPort: layers.UDPPort(dstPort),
	}

	switch ipVersion {
	case IPv4:
		eth = &layers.Ethernet{
			SrcMAC:       srcMac,
			DstMAC:       dstMac,
			EthernetType: layers.EthernetTypeIPv4,
		}
		ip4 = &layers.IPv4{
			Version:  4,
			IHL:      5,
			TTL:      64,
			Id:       0,
			Protocol: layers.IPProtocolUDP,
			SrcIP:    srcIP,
			DstIP:    dstIP,
		}

		err := udp.SetNetworkLayerForChecksum(ip4)
		if err != nil {
			return nil, err
		}

		err = gopacket.SerializeLayers(buf, opts, eth, ip4, udp, gopacket.Payload(payload))
		if err != nil {
			return nil, err
		}

	case IPv6:
		eth = &layers.Ethernet{
			SrcMAC:       srcMac,
			DstMAC:       dstMac,
			EthernetType: layers.EthernetTypeIPv6,
		}
		ip6 = &layers.IPv6{
			BaseLayer: layers.BaseLayer{},
			Version:   6,
			HopLimit:  64,
			SrcIP:     srcIP,
			DstIP:     dstIP,
		}

		err := udp.SetNetworkLayerForChecksum(ip6)
		if err != nil {
			return nil, err
		}

		err = gopacket.SerializeLayers(buf, opts, eth, ip6, udp, gopacket.Payload(payload))
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func GetFreeUDPPort() (port uint16, err error) {
	var conn *net.UDPConn
	var addrPort netip.AddrPort
	var laddr net.UDPAddr = net.UDPAddr{
		IP:   nil,
		Port: 0,
	}

	port = 0

	if conn, err = net.ListenUDP("udp", &laddr); err != nil {
		return
	}

	addrPort, err = netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		return
	}

	return addrPort.Port(), nil
}
