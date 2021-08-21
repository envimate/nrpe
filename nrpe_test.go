package nrpe

import (
	"net"
	"os"
	"runtime"
	"syscall"

	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"encoding/binary"
)

func ExampleRun() {
	conn, err := net.Dial("tcp", "127.0.0.1:5666")
	if err != nil {
		fmt.Println(err)
		return
	}

	command := NewCommand("check_load")

	// ssl = true, timeout = 0
	result, err := Run(conn, command, true, 0)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(result.StatusLine)
	os.Exit(int(result.StatusCode))
}

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

func TestClientServer(t *testing.T) {

	sock := testCreateSocketPair(t)

	c := make(chan int)

	go func() {
		err := ServeOne(sock.server, func(command Command) (*CommandResult, error) {
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, false, 0)

		if err != nil {
			t.Fatal(err)
		}

		c <- 1
	}()

	command := NewCommand("check_something", "1", "2")

	result, err := Run(sock.client, command, false, 0)

	if err != nil {
		t.Fatal(err)
	}

	if result.StatusLine != ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")) {
		t.Fatal("Unexpected response")
	}

	<-c
}

func TestClientServerTimeoutOk(t *testing.T) {
	sock := testCreateSocketPair(t)

	c := make(chan int)

	go func() {
		err := ServeOne(sock.server, func(command Command) (*CommandResult, error) {
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, false, 0)

		if err != nil {
			t.Fatal(err)
		}

		c <- 1
	}()

	command := NewCommand("check_something", "1", "2")

	result, err := Run(sock.client, command, false, 5*time.Second)

	if err != nil {
		t.Fatal(err)
	}

	if result.StatusLine != ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")) {
		t.Fatal("Unexpected response")
	}

	<-c
}

func TestClientServerTimeoutServer(t *testing.T) {
	sock := testCreateSocketPair(t)

	c := make(chan int)

	go func() {
		err := ServeOne(sock.server, func(command Command) (*CommandResult, error) {
			time.Sleep(10)
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, false, 10)

		if err == nil {
			t.Fatal("Expected timeout")
		}

		c <- 1
	}()

	command := NewCommand("check_something", "1", "2")

	result, err := Run(sock.client, command, false, 1)

	if err == nil || result != nil {
		t.Fatal("Expected timeout")
	}

	<-c
}

func TestClientServerTimeoutClient(t *testing.T) {
	sock := testCreateSocketPair(t)

	c := make(chan int)

	go func() {
		err := ServeOne(sock.server, func(command Command) (*CommandResult, error) {
			return &CommandResult{
				StatusLine: ("CMD=" + command.Name + " ARGS=" + strings.Join(command.Args, ",")),
				StatusCode: StatusOK,
			}, nil
		}, false, 1)

		if err == nil {
			t.Fatal("Expected timeout")
		}

		c <- 1
	}()

	time.Sleep(10)

	<-c
}

func TestRandSource(t *testing.T) {
	const c = 1000000
	var wg sync.WaitGroup
	wg.Add(c)

	for i := 0; i < c; i++ {
		go func(i int) {
			defer wg.Done()
			randSource.Seed(0)
			randSource.Uint32()
		}(i)
	}
	wg.Wait()
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
			t.Fatal("Rand result didn't match")
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
	read  func([]byte) (n int, err error)
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

	command := NewCommand("check_something", "1", "2")

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "you shall not pass" {
		t.Fatal("Expecting error")
	}
}

func TestClientReadError(t *testing.T) {
	sock := testCreateSocketPair(t)

	clientSock := &testConn{Conn: sock.client}

	clientSock.read = func(b []byte) (n int, err error) {
		return -1, fmt.Errorf("you shall not pass")
	}

	command := NewCommand("check_something", "1", "2")

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "you shall not pass" {
		t.Fatal("Expecting error")
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

	command := NewCommand("check_something", "1", "2")

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "nrpe: Error response packet type, got: 1, expected: 2" {
		t.Fatal("Expecting error")
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

	command := NewCommand("check_something", "1", "2")

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "nrpe: Response crc didn't match" {
		t.Fatal("Expecting error")
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

	command := NewCommand("check_something", "1", "2")

	_, err := Run(clientSock, command, false, 0)

	if err == nil || err.Error() != "nrpe: Unknown status code 10" {
		t.Fatal("Expecting error")
	}
}

func TestClientLongStatusLineError(t *testing.T) {
	sock := testCreateSocketPair(t)

	command := NewCommand(strings.Repeat("a", 2048))

	_, err := Run(sock.client, command, false, 0)

	if err == nil || err.Error() != "nrpe: Command is too long: got 2048, max allowed 1023" {
		t.Fatal("Expected error")
	}
}

func TestPacketTruncation(t *testing.T) {
	statusLine := make([]byte, 2048)

	for i := 0; i < len(statusLine); i++ {
		statusLine[i] = byte(i & 0xFF)
	}

	packet := buildPacket(0, 0, statusLine)

	statusLine[len(packet.data)-1] = 0

	if !bytes.Equal(packet.data, statusLine[:len(packet.data)]) {
		t.Fatal("Data modified after truncation")
	}
}

func TestPacketTruncatedIO(t *testing.T) {
	sock := testCreateSocketPair(t)

	clientSock := &testConn{Conn: sock.client}

	clientSock.write = func(b []byte) (int, error) {
		return len(b) / 2, nil
	}

	clientSock.read = clientSock.write

	packet := createPacket()

	err := writePacket(clientSock, 0, packet)

	if err == nil || err.Error() != "nrpe: error while writing" {
		t.Fatal("Expecting an error")
	}

	err = readPacket(clientSock, 0, packet)

	if err == nil || err.Error() != "nrpe: error while reading" {
		t.Fatal("Expecting an error")
	}
}

func TestServerReadError(t *testing.T) {
	sock := testCreateSocketPair(t)

	serverSock := &testConn{Conn: sock.server}

	serverSock.read = func(b []byte) (n int, err error) {
		return -1, fmt.Errorf("you shall not pass")
	}

	err := ServeOne(serverSock, nil, false, 0)

	if err == nil || err.Error() != "you shall not pass" {
		t.Fatal("Expecting error")
	}
}

func TestServerVerifyTypeError(t *testing.T) {
	sock := testCreateSocketPair(t)

	serverSock := &testConn{Conn: sock.client}

	serverSock.read = func(b []byte) (n int, err error) {
		p := buildPacket(responsePacketType, 0, []byte("test"))
		copy(b, p.all)
		return len(p.all), nil
	}

	err := ServeOne(serverSock, nil, false, 0)

	if err == nil || err.Error() != "nrpe: Error response packet type, got: 2, expected: 1" {
		t.Fatal("Expecting error")
	}
}

func TestServerVerifyCrcError(t *testing.T) {
	sock := testCreateSocketPair(t)

	serverSock := &testConn{Conn: sock.client}

	serverSock.read = func(b []byte) (n int, err error) {
		p := buildPacket(queryPacketType, 0, []byte("test"))

		p.crc32[0] = 0

		copy(b, p.all)

		return len(p.all), nil
	}

	err := ServeOne(serverSock, nil, false, 0)

	if err == nil || err.Error() != "nrpe: Response crc didn't match" {
		t.Fatal("Expecting error")
	}
}

func TestServerInvalidRespone(t *testing.T) {
	sock := testCreateSocketPair(t)

	serverSock := &testConn{Conn: sock.client}

	serverSock.read = func(b []byte) (n int, err error) {
		be := binary.BigEndian

		p := createPacket()

		be.PutUint16(p.packetVersion, nrpePacketVersion2)
		be.PutUint16(p.packetType, queryPacketType)
		be.PutUint32(p.crc32, 0)
		be.PutUint16(p.statusCode, 0)

		copy(p.data, bytes.Repeat([]byte("A"), len(p.data)))
		be.PutUint32(p.crc32, crc32(p.all))

		copy(b, p.all)

		return len(p.all), nil
	}

	err := ServeOne(serverSock, nil, false, 0)

	if err == nil || err.Error() != "nrpe: invalid request" {
		t.Fatal("Expecting error")
	}
}

func TestServerHandlerError(t *testing.T) {
	sock := testCreateSocketPair(t)

	serverSock := &testConn{Conn: sock.client}

	serverSock.read = func(b []byte) (n int, err error) {
		p := buildPacket(queryPacketType, 0, []byte("test"))

		copy(b, p.all)

		return len(p.all), nil
	}

	err := ServeOne(serverSock, func(Command) (*CommandResult, error) {
		return nil, fmt.Errorf("you shall not pass")
	}, false, 0)

	if err == nil || err.Error() != "you shall not pass" {
		t.Fatal("Expecting error")
	}
}

func TestServerWriteError(t *testing.T) {
	sock := testCreateSocketPair(t)

	serverSock := &testConn{Conn: sock.client}

	serverSock.read = func(b []byte) (n int, err error) {
		p := buildPacket(queryPacketType, 0, []byte("test"))

		copy(b, p.all)

		return len(p.all), nil
	}

	serverSock.write = func(b []byte) (n int, err error) {
		return -1, fmt.Errorf("you shall not pass")
	}

	err := ServeOne(serverSock, func(Command) (*CommandResult, error) {
		return &CommandResult{
			StatusLine: "test",
			StatusCode: StatusOK,
		}, nil
	}, false, 0)

	if err == nil || err.Error() != "you shall not pass" {
		t.Fatal("Expecting error")
	}
}
