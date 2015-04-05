package main

import (
	"errors"
	"github.com/zmap/zgrab/zlib"
	"net"
	"testing"
)

func TestSimplifyTimeoutError(t *testing.T) {
	oldErr := errors.New("Could not connect to  remote host 131.87.2.40: dial tcp 131.87.2.40:25: i/o timeout")
	newErr := simplfiyError(oldErr)

	if newErr.Error() != "i/o timeout" {
		t.Fatal("invalid return value: ", newErr)
	}
}

func TestSimplifyStarttlsError(t *testing.T) {
	oldErr := errors.New("Conversation error with remote host 207.58.132.103:25: Bad return code for STARTTLS")
	newErr := simplfiyError(oldErr)

	if newErr.Error() != "Bad return code for STARTTLS" {
		t.Fatal("invalid return value: ", newErr)
	}
}

func TestTimeout(t *testing.T) {
	target := zlib.GrabTarget{Addr: net.ParseIP("192.168.254.254")}
	result := NewZgrabResult(target)

	if result.Error.Error() != "i/o timeout" {
		t.Fatal("an unexpected error occured:", result.Error)
	}

	if result.HasStarttls() != false {
		t.Fatal("host should not have starttls")
	}

}

func TestWithStarttls(t *testing.T) {
	target := zlib.GrabTarget{Addr: net.ParseIP("109.69.71.161")}
	result := NewZgrabResult(target)

	if result.HasStarttls() != true {
		t.Fatal("host should not have")
	}

	if result.Error != nil {
		t.Fatal("an error occured: ", result.Error)
	}

}

func TestWithoutStarttls(t *testing.T) {
	target := zlib.GrabTarget{Addr: net.ParseIP("198.23.62.105")}
	result := NewZgrabResult(target)

	if result.HasStarttls() != false {
		t.Fatal("host should not have starttls")
	}

}
