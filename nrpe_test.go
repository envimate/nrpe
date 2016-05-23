package nrpe

import (
	"net"
	"os"
	"runtime"
	"syscall"

	"bytes"
	"strings"
	"fmt"
	"testing"
	"unsafe"
)

type testSocketPair struct {
	clientFile *os.File
	serverFile *os.File
	client     net.Conn
	server     net.Conn
}

func (s *testSocketPair) Close() {
	s.clientFile.Close()
	s.serverFile.Close()
	s.client.Close()
	s.server.Close()
}

func testCreateSocketPair(t *testing.T) *testSocketPair {
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

	s := &testSocketPair{
		clientFile: clientFile,
		serverFile: serverFile,
		client:     clientConn,
		server:     serverConn,
	}

	runtime.SetFinalizer(s, func(s *testSocketPair) {
		s.Close()
	})

	return s
}

func TestBufferRandomizer(t *testing.T) {
	randSource.Seed(0)

	randExpected := make([]byte, 8)

	r := randSource.Uint32()
	copy(randExpected[:4], (*[4]byte)(unsafe.Pointer(&r))[:])
	r = randSource.Uint32()
	copy(randExpected[4:], (*[4]byte)(unsafe.Pointer(&r))[:])

	for i := 0; i < len(randExpected); i++ {
		randSource.Seed(0)

		buf := make([]byte, i)

		randomizeBuffer(buf)

		if !bytes.Equal(buf, randExpected[:i]) {
			t.Fatal("rand result didn't match")
		}
	}
}

func TestCommandToStatusLine(t *testing.T) {
	if NewCommand("commandName", "arg1", "arg2").toStatusLine() !=
		"commandName!arg1!arg2" {

		t.Fatal("toStatusLine with arguments returned wrong result")
	}

	if NewCommand("commandName").toStatusLine() != "commandName" {
		t.Fatal("toStatusLine without arguments returned wrong result")
	}
}

type testConn struct {
	net.Conn
	read func([]byte) (n int, err error)
	write func([]byte) (n int, err error)
}

func (c testConn) Read(b []byte) (n int, err error) {
	if c.read != nil {
		return c.read(b)
	}
	return c.Conn.Read(b)
}

func (c testConn) Write(b []byte) (n int, err error) {
	if c.write != nil {
		return c.write(b)
	}
	return c.Conn.Write(b)
}


func TestClientWriteError(t *testing.T) {
	sock := testCreateSocketPair(t)

	clientSock := &testConn{Conn: sock.client}

	clientSock.write = func(b []byte) (n int, err error) {
		return -1, fmt.Errorf("you shall not pass")
	}

	command := NewCommand("check_bla", "1", "2")

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "you shall not pass" {
		t.Fatal("Expected error")
	}
}

func TestClientReadError(t *testing.T) {
	sock := testCreateSocketPair(t)

	clientSock := &testConn{Conn: sock.client}

	clientSock.read = func(b []byte) (n int, err error) {
		return -1, fmt.Errorf("you shall not pass")
	}

	command := NewCommand("check_bla", "1", "2")

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "you shall not pass" {
		t.Fatal("Expected error")
	}
}

func TestClientVerifyTypeError(t *testing.T) {
	sock := testCreateSocketPair(t)

	clientSock := &testConn{Conn: sock.client}

	clientSock.read = func(b []byte) (n int, err error) {
		p := buildPacket(queryPacketType, 0, []byte("test"))
		copy(b, p.all)
		return len(p.all), nil
	}

	command := NewCommand("check_bla", "1", "2")

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "nrpe: Error response packet type, got: 1, expected: 2" {
		t.Fatal("Expected error")
	}
}

func TestClientVerifyCrcError(t *testing.T) {
	sock := testCreateSocketPair(t)

	clientSock := &testConn{Conn: sock.client}

	clientSock.read = func(b []byte) (n int, err error) {
		p := buildPacket(responsePacketType, 0, []byte("test"))

		p.crc32[0] = 0

		copy(b, p.all)

		return len(p.all), nil
	}

	command := NewCommand("check_bla", "1", "2")

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "nrpe: Response crc didn't match" {
		t.Fatal("Expected error")
	}
}

func TestClientStatusError(t *testing.T) {
	sock := testCreateSocketPair(t)

	clientSock := &testConn{Conn: sock.client}

	clientSock.read = func(b []byte) (n int, err error) {
		p := buildPacket(responsePacketType, 10, []byte("test"))

		copy(b, p.all)

		return len(p.all), nil
	}

	command := NewCommand("check_bla", "1", "2")

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "nrpe: Unknown status code 10" {
		t.Fatal("Expected error")
	}
}

func TestClientLongStatusLineError(t *testing.T) {
	sock := testCreateSocketPair(t)

	clientSock := &testConn{Conn: sock.client}

	command := NewCommand(strings.Repeat("a", 2048))

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "nrpe: Command is too long: got 2048, max allowed 1023" {
		t.Fatal("Expected error")
	}
}

