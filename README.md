# nrpe

Package `envimate/nrpe` implements NRPE client library for go.

You will need libssl, in order to use this library.

Example

```go
package main

import (
        "fmt"
        "github.com/envimate/nrpe"
        "net"
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

        fmt.Printf("%s\n", result.StatusLine)
}
```
