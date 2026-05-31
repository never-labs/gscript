package net

import stdnet "net"

func ConnectAddr(addr string) string {
	host, port, err := stdnet.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "::", "[::]", "0.0.0.0":
		host = "127.0.0.1"
	}
	return stdnet.JoinHostPort(host, port)
}
