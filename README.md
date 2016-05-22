# nrpe client/server library

[![GoDoc](https://godoc.org/github.com/envimate/nrpe?status.svg)](https://godoc.org/github.com/envimate/nrpe)
[![Build Status](https://travis-ci.org/envimate/nrpe.svg?branch=master)](https://travis-ci.org/envimate/nrpe)

Package `envimate/nrpe` implements NRPE client/server library for go.

It supports plain and ssl modes and fully compatible with standard nrpe library.

Package includes `check_nrpe` command, which is alternate implementation of homonymous command shipped with nrpe package.

Requires libssl to compile and run.

## Client Example

```go
package main

import (
        "fmt"
        "github.com/envimate/nrpe"
        "net"
        "os"
)

func main() {
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
```

## Server Example

```go
package main

import (
	"fmt"
	"net"

	"github.com/envimate/nrpe"
)

func nrpeHandler(c nrpe.Command) (*nrpe.CommandResult, error) {
	// handle nrpe command here

	return &nrpe.CommandResult{
		StatusLine: "COMMAND=" + c.Name,
		StatusCode: nrpe.StatusOK,
	}, nil
}

func connectionHandler(conn net.Conn) {
	defer conn.Close()
	nrpe.ServeOne(conn, nrpeHandler, true, 0)
}

func main() {
	ln, err := net.Listen("tcp", ":5667")

	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		conn, err := ln.Accept()

		if err != nil {
			fmt.Println(err)
			continue
		}

		go connectionHandler(conn)
	}
}
```

## Checkout and compile check_nrpe

### checkout
`go get github.com/envimate/nrpe`

### compile
`go build github.com/envimate/nrpe/cmd/check_nrpe`
