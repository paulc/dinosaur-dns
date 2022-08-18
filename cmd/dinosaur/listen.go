package main

import (
	"net"
	"regexp"
	"strings"
)

// We accept listen addresses as either:
//
//    ip:port
//    [ip6]:port
//    ip 				(port defaults to :53)
//    ip6		    	(port defaults to :53)
//    interface:port	(all addresses on interface)
//    interface			(all addresses on interface - port :53)

func parseListenAddr(s string) (addrs []string, err error) {

	portRE := regexp.MustCompile(`:\d+$`)

	if !portRE.MatchString(s) {
		// Append default port
		s += ":53"
	}

	split := strings.SplitN(s, ":", 2)
	name, port := split[0], split[1]

	// Check if this is a valid interface name
	netif, err := net.InterfaceByName(name)
	if err != nil {
		// Assume IP or Hostname
		addrs = append(addrs, s)
		return addrs, nil
	}

	// Get addresses
	ifaddrs, err := netif.Addrs()
	if err != nil {
		return nil, err
	}
	for _, v := range ifaddrs {
		ip, _, err := net.ParseCIDR(v.String())
		if err != nil {
			return nil, err
		}
		if ip.IsLinkLocalUnicast() {
			addrs = append(addrs, net.JoinHostPort(ip.String()+"%"+netif.Name, port))
		} else {
			addrs = append(addrs, net.JoinHostPort(ip.String(), port))
		}
	}
	return
}

func isV4Global(hostport string) bool {
	host, _, _ := net.SplitHostPort(hostport)
	return host == "0.0.0.0" || host == "0"
}

func isV6Global(hostport string) bool {
	host, _, _ := net.SplitHostPort(hostport)
	return host == "::" || host == "::0"
}

/*
func main() {
	for _, v := range os.Args[1:] {
		fmt.Println("====", v)
		addrs, err := parseListenAddr(v)
		if err == nil {
			for _, v := range addrs {
				fmt.Println(v, isV4Global(v), isV6Global(v))
			}
		} else {
			fmt.Println(err)
		}
	}
}
*/
