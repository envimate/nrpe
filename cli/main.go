package main

import (
	"flag"
	"fmt"
	"github.com/envimate/nrpe"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	var cmd, host string
	var port int
	var isSsl bool
	var timeout time.Duration

	cmdFlag := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	cmdFlag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage of %s: [options] [--] [arglist]\nOptions:\n", os.Args[0])
		cmdFlag.PrintDefaults()
	}

	cmdFlag.StringVar(&host, "host", "127.0.0.1", "hostname to connect")
	cmdFlag.IntVar(&port, "port", 5666, "port number")
	cmdFlag.BoolVar(&isSsl, "ssl", true, "use ssl")
	cmdFlag.StringVar(&cmd, "command", "version", "command to execute")
	cmdFlag.DurationVar(&timeout, "timeout", 0, "network timeout")

	cmdFlag.Parse(os.Args[1:])

	conn, err := net.DialTimeout(
		"tcp",
		net.JoinHostPort(host, strconv.Itoa(port)),
		5*time.Second,
	)

	if err != nil {
		fmt.Printf("nrpe: error while connecting %s\n", err)
		os.Exit(int(nrpe.StatusUnknown))
	}

	args := cmdFlag.Args()

	command := nrpe.NewCommand(cmd, args...)

	result, err := nrpe.Run(conn, command, isSsl, timeout)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(int(nrpe.StatusUnknown))
	}

	fmt.Printf("%s\n", result.StatusLine)
	os.Exit(int(result.StatusCode))
}
