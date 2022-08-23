package util

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// Accept addresses as either:
//
//    ip:port
//    [ip6]:port
//    ip 				(default port)
//    ip6		    	(default port)
//    interface:port	(all addresses on interface)
//    interface			(all addresses on interface - default port)
//
// Returns list of ip:port addrs suitable for net.Listen

func ParseAddr(addr string, defaultPort int) (addrs []string, err error) {

	portRE := regexp.MustCompile(`:\d+$`)

	if !portRE.MatchString(addr) {
		// Append default port
		addr = fmt.Sprintf("%s:%d", addr, defaultPort)
	}

	split := strings.SplitN(addr, ":", 2)
	name, port := split[0], split[1]

	// Check if this is a valid interface name
	netif, err := net.InterfaceByName(name)
	if err != nil {
		// Assume IP or Hostname
		addrs = append(addrs, addr)
		return addrs, nil
	}

	// Get addresses
	ifaddrs, err := netif.Addrs()
	if err != nil {
		return nil, fmt.Errorf("Error getting interface addresses: %s", err)
	}
	for _, v := range ifaddrs {
		ip, _, err := net.ParseCIDR(v.String())
		if err != nil {
			return nil, fmt.Errorf("Error parsing addresse <%s>: %s", v.String(), err)
		}
		if ip.IsLinkLocalUnicast() {
			addrs = append(addrs, net.JoinHostPort(ip.String()+"%"+netif.Name, port))
		} else {
			addrs = append(addrs, net.JoinHostPort(ip.String(), port))
		}
	}
	return
}
