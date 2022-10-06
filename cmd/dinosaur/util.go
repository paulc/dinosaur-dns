package main

import "net"

func AclToString(acl []net.IPNet) (out []string) {
	for _, v := range acl {
		out = append(out, v.String())
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
