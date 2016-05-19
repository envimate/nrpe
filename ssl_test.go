package nrpe

import (
	"testing"
	"strings"
	"syscall"
	"os"
	"net"
	"fmt"
)

func TestClientServer(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM, 0)

	if err != nil {
		t.Fatal(err)
		return
	}

	clientFile := os.NewFile(uintptr(fds[0]), "client")
	serverFile := os.NewFile(uintptr(fds[1]), "server")

	clientConn, err := net.FileConn(clientFile)

	if err != nil {
		t.Fatal(err)
		return
	}

	serverConn, err := net.FileConn(serverFile)

	if err != nil {
		t.Fatal(err)
		return
	}

	go func() {
		err := ServeOne(serverConn, func(command Command) (*CommandResult, error) {
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, true, 0)

		if err != nil {
			t.Fatal(err)
		}
	} ()

	command := NewCommand("check_bla", "1", "2")

	result, err := Run(clientConn, command, true, 0)

	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("%+v\n", result)
}
