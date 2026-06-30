package dhcp

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/mdlayher/arp"
)

// probeConflict sends an RFC 5227 ARP probe (sender IP = 0.0.0.0) for ip on
// iface and returns true if another host replies within 500 ms.
// Returns (false, nil) if no conflict is detected or if the probe cannot be
// sent (e.g. insufficient privileges) so that the caller can proceed.
func probeConflict(iface *net.Interface, ip net.IP) (conflict bool, err error) {
	client, err := arp.Dial(iface)
	if err != nil {
		return false, nil
	}
	defer client.Close()

	client.SetDeadline(time.Now().Add(500 * time.Millisecond))

	ip4 := ip.To4()
	if ip4 == nil {
		return false, nil
	}
	targetAddr, ok := netip.AddrFromSlice(ip4)
	if !ok {
		return false, nil
	}
	targetAddr = targetAddr.Unmap()

	probe, err := arp.NewPacket(
		arp.OperationRequest,
		iface.HardwareAddr,
		netip.Addr{}, // sender IP = 0.0.0.0 per RFC 5227
		net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		targetAddr,
	)
	if err != nil {
		return false, nil
	}

	if err := client.WriteTo(probe, net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}); err != nil {
		return false, nil
	}

	for {
		reply, _, err := client.Read()
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return false, nil
			}
			return false, nil
		}
		if reply.Operation == arp.OperationReply && reply.SenderIP == targetAddr {
			return true, nil
		}
	}
}
