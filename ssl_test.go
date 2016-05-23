package nrpe

import (
	"strings"
	"testing"
	"time"
)

func TestClientServerSsl(t *testing.T) {

	sock := testCreateSocketPair(t)

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
	sock := testCreateSocketPair(t)

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
	sock := testCreateSocketPair(t)

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
			t.Fatal("Expected timeout ")
		}

		c <- 1
	}()

	time.Sleep(10)

	<-c
}
