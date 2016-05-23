package nrpe

import (
	_ "fmt"
	"strings"
	"testing"
	"time"
)

func TestClientServerSsl(t *testing.T) {

	sock := testCreateSocketPair(t)

	c := make(chan int)

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

		c <- 1
	}()

	command := NewCommand("check_something", "1", "2")

	result, err := Run(sock.client, command, true, 0)

	if err != nil {
		t.Fatal(err)
	}

	if result.StatusLine != ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")) {
		t.Fatal("Unexpected response")
	}

	<-c
}

func TestClientServerSslTimeoutOk(t *testing.T) {
	sock := testCreateSocketPair(t)

	c := make(chan int)

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

		c <- 1
	}()

	command := NewCommand("check_something", "1", "2")

	result, err := Run(sock.client, command, true, 5*time.Second)

	if err != nil {
		t.Fatal(err)
	}

	if result.StatusLine != ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")) {
		t.Fatal("Unexpected response")
	}

	<-c
}

func TestClientServerSslTimeoutServer(t *testing.T) {
	sock := testCreateSocketPair(t)

	c := make(chan int)

	go func() {
		err := ServeOne(sock.server, func(command Command) (*CommandResult, error) {
			time.Sleep(10)
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, true, 10)

		if err == nil {
			t.Fatal("Expected timeout")
		}

		c <- 1
	}()

	command := NewCommand("check_something", "1", "2")

	result, err := Run(sock.client, command, true, 1)

	if err == nil || result != nil {
		t.Fatal("Expected timeout")
	}

	<-c
}

func TestClientServerSslTimeoutClient(t *testing.T) {
	sock := testCreateSocketPair(t)

	c := make(chan int)

	go func() {
		err := ServeOne(sock.server, func(command Command) (*CommandResult, error) {
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, true, 1)

		if err == nil {
			t.Fatal("Expected timeout")
		}

		c <- 1
	}()

	time.Sleep(10)

	<-c
}

func TestSslWritePanic(t *testing.T) {
	sock := testCreateSocketPair(t)

	clientSock := &testConn{Conn: sock.client}

	clientSock.write = func(b []byte) (n int, err error) {
		panic("you shall not pass")
		return -1, nil
	}

	cl, err := newSSLClient(clientSock)

	if err != nil {
		t.Fatal(err)
	}

	_, err = cl.Write(make([]byte, 1))

	if err == nil || !strings.HasPrefix(err.Error(), "nrpe: error on ssl handshake") {
		t.Fatal("Expected error")
	}
}

func TestSslReadPanic(t *testing.T) {
	sock := testCreateSocketPair(t)

	serverSock := &testConn{Conn: sock.client}

	serverSock.read = func(b []byte) (n int, err error) {
		panic("you shall not pass")
		return -1, nil
	}

	sl, err := newSSLServerConn(serverSock)

	if err != nil {
		t.Fatal(err)
	}

	_, err = sl.Read(make([]byte, 1))

	if err == nil || !strings.HasPrefix(err.Error(), "nrpe: error on ssl handshake") {
		t.Fatal("Expected error")
	}
}

func TestSslReadError(t *testing.T) {
	sock := testCreateSocketPair(t)

	sl, err := newSSLServerConn(sock.server)

	if err != nil {
		t.Fatal(err)
	}

	connMap.lock.Lock()
	for k := range connMap.values {
		delete(connMap.values, k)
	}
	connMap.lock.Unlock()

	_, err = sl.Read(make([]byte, 1))

	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestSslWriteError(t *testing.T) {
	sock := testCreateSocketPair(t)

	cl, err := newSSLClient(sock.client)

	if err != nil {
		t.Fatal(err)
	}

	connMap.lock.Lock()
	for k := range connMap.values {
		delete(connMap.values, k)
	}
	connMap.lock.Unlock()

	_, err = cl.Write(make([]byte, 1))

	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestSslCloseError(t *testing.T) {
	sock := testCreateSocketPair(t)

	cl, err := newSSLClient(sock.client)

	if err != nil {
		t.Fatal(err)
	}

	if len(connMap.values) != 1 {
		t.Fatal("connection map contains too many elements")
	}

	cl.Close()

	if len(connMap.values) != 0 {
		t.Fatal("connection map must be empty")
	}
}

func TestSslWriteStateError(t *testing.T) {
	sock := testCreateSocketPair(t)

	cl, err := newSSLClient(sock.client)

	if err != nil {
		t.Fatal(err)
	}

	cl.(*sslConn).state = stateError

	_, err = cl.Write(make([]byte, 1))

	if err == nil || !strings.HasPrefix(err.Error(), "nrpe: inconsistent connection state") {
		t.Fatal("Expected error")
	}
}

func TestSslReadStateError(t *testing.T) {
	sock := testCreateSocketPair(t)

	cl, err := newSSLClient(sock.client)

	if err != nil {
		t.Fatal(err)
	}

	cl.(*sslConn).state = stateError

	_, err = cl.Read(make([]byte, 1))

	if err == nil || !strings.HasPrefix(err.Error(), "nrpe: inconsistent connection state") {
		t.Fatal("Expected error")
	}
}
