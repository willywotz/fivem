package main

import (
	"net"
)

func main() {
	serverAddr, _ := net.ResolveUDPAddr("udp", "192.168.6.4:8080")
	conn, _ := net.DialUDP("udp", nil, serverAddr)
	defer func() { _ = conn.Close() }()
}
