package nrpe

import (
	. "fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

type socketPair struct {
	clientFile *os.File
	serverFile *os.File
	client     net.Conn
	server     net.Conn
}

func (s *socketPair) Close() {
	s.clientFile.Close()
	s.serverFile.Close()
	s.client.Close()
	s.server.Close()
}

func createSocketPair(t *testing.T) *socketPair {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM, 0)

	if err != nil {
		t.Fatal(err)
	}

	clientFile := os.NewFile(uintptr(fds[0]), "client")
	serverFile := os.NewFile(uintptr(fds[1]), "server")

	defer clientFile.Close()
	defer serverFile.Close()

	clientConn, err := net.FileConn(clientFile)

	if err != nil {
		t.Fatal(err)
	}

	serverConn, err := net.FileConn(serverFile)

	if err != nil {
		t.Fatal(err)
	}

	s := &socketPair{
		clientFile: clientFile,
		serverFile: serverFile,
		client:     clientConn,
		server:     serverConn,
	}

	runtime.SetFinalizer(s, func(s *socketPair) {
		Println("pair Finalizer")
		s.Close()
	})

	return s
}

func TestClientServerSsl(t *testing.T) {

	sock := createSocketPair(t)

	go func() {
		err := ServeOne(sock.server, func(command Command) (*CommandResult, error) {
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, true, 0)

		if err != nil {
			t.Fatal(err)
		}
	}()

	command := NewCommand("check_bla", "1", "2")

	result, err := Run(sock.client, command, true, 0)

	if err != nil {
		t.Fatal(err)
	}

	if result.StatusLine != ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")) {
		t.Fatal("Unexpected response")
	}
}

func TestClientServerSslTimeoutOk(t *testing.T) {
	sock := createSocketPair(t)

	go func() {
		err := ServeOne(sock.server, func(command Command) (*CommandResult, error) {
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, true, 0)

		if err != nil {
			t.Fatal(err)
		}
	}()

	command := NewCommand("check_bla", "1", "2")

	result, err := Run(sock.client, command, true, 5*time.Second)

	if err != nil {
		t.Fatal(err)
	}

	if result.StatusLine != ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")) {
		t.Fatal("Unexpected response")
	}
}

func TestClientServerSslTimeoutServer(t *testing.T) {
	sock := createSocketPair(t)

	go func() {
		err := ServeOne(sock.server, func(command Command) (*CommandResult, error) {
			time.Sleep(10)
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, true, 0)

		if err != nil {
			t.Fatal(err)
		}
	}()

	command := NewCommand("check_bla", "1", "2")

	result, err := Run(sock.client, command, true, 1)

	if err == nil || result != nil {
		t.Fatal("Expected timeout")
	}
}

func TestClientServerSslTimeoutClient(t *testing.T) {
	sock := createSocketPair(t)

	c := make(chan int)

	go func() {
		err := ServeOne(sock.server, func(command Command) (*CommandResult, error) {
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, true, 1)

		if err == nil {
			t.Fatal("Expected timeout ")
		}

		c <- 1
	}()

	time.Sleep(10)

	<-c
}
