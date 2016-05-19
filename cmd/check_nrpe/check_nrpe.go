/*

check_nrpe is a command line NRPE client.

Calls the remote nrpe server and returns the response and status code.
In case of connectivity issues with NRPE server the error is written into Stderr
and the status code is 3.


Usage:
	nrpe: [flag] [--] [arglist]

The flags are:
	-command string
		command to execute (default "version")
	-host string
		hostname to connect (default "127.0.0.1")
	-port int
		port number (default 5666)
	-ssl
		use ssl (default true)
	-timeout duration
		network timeout


*/
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/envimate/nrpe"
)

func main() {
	var cmd, host string
	var port int
	var isSSL bool
	var timeout time.Duration

	cmdFlag := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	cmdFlag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage of %s: [options] [--] [arglist]\nOptions:\n", os.Args[0])
		cmdFlag.PrintDefaults()
	}

	cmdFlag.StringVar(&host, "host", "127.0.0.1", "hostname to connect")
	cmdFlag.IntVar(&port, "port", 5666, "port number")
	cmdFlag.BoolVar(&isSSL, "ssl", true, "use ssl")
	cmdFlag.StringVar(&cmd, "command", "version", "command to execute")
	cmdFlag.DurationVar(&timeout, "timeout", 0, "network timeout")

	cmdFlag.Parse(os.Args[1:])

	conn, err := net.DialTimeout(
		"tcp",
		net.JoinHostPort(host, strconv.Itoa(port)),
		timeout,
	)

	if err != nil {
		fmt.Printf("nrpe: error while connecting %s\n", err)
		os.Exit(int(nrpe.StatusUnknown))
	}

	args := cmdFlag.Args()

	command := nrpe.NewCommand(cmd, args...)

	result, err := nrpe.Run(conn, command, isSSL, timeout)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(int(nrpe.StatusUnknown))
	}

	fmt.Printf("%s\n", result.StatusLine)
	os.Exit(int(result.StatusCode))
}
