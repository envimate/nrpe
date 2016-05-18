package main

import (
	_ "crypto/tls"
	"fmt"
	nrpe "github.com/envimate/nrpe/client"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:5666")
	//conn, err := tls.Dial("tcp", "127.0.0.1:5666")

	if err != nil {
		fmt.Printf("error connectiog %s", err)
		return
	}

	_ = conn

	//packet := nrpe.CreatePacket("check_load")

	res, err := nrpe.SendRequest(conn, "check_load")

	fmt.Printf("%s\n", *res)

	/*err = nrpe.SendRequest(conn, "check_load")

	fmt.Printf("%s\n", err)*/

	//nrpe.SendPacket(conn, packet)

	// fmt.Printf("%+v\n", packet)
}
