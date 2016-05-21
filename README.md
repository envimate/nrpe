# nrpe client/server library

Package `envimate/nrpe` implements NRPE client library for go.

Requires libssl to compile and run.

### Status
[![Build Status](https://travis-ci.org/envimate/nrpe.svg?branch=master)](https://travis-ci.org/envimate/nrpe)

## Example

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

## Checkout and compile

### checkout
`go get github.com/envimate/nrpe`

### compile

`go build github.com/envimate/nrpe/cmd/check_nrpe`
