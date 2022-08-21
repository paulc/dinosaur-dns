package main

import (
	"net"
)

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
