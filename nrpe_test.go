package nrpe_test

import (
	"fmt"
	"net"
	"os"

	"github.com/envimate/nrpe"
)

func ExampleRun() {
	conn, err := net.Dial("tcp", "127.0.0.1:5666")
	if err != nil {
		fmt.Println(err)
		return
	}

	command := nrpe.NewCommand("check_load")

	// ssl = true, timeout = 0
	result, err := nrpe.Run(conn, command, true, 0)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(result.StatusLine)
	os.Exit(int(result.StatusCode))
}
