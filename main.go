package main

import (
	"net"
	_ "crypto/tls"
	"fmt"
	nrpe "github.com/envimate/nrpe/client"
)

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:5666")
	//conn, err := tls.Dial("tcp", "127.0.0.1:5666")

	if err != nil {
		fmt.Printf("error connectiog %s", err)
		return
	}

	_ = conn

	packet := nrpe.CreatePacket("check_load")

	nrpe.SendPacket(conn, packet)

	// fmt.Printf("%+v\n", packet)
}
