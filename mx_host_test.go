package main

import (
	"errors"
	"github.com/zmap/zgrab/zlib"
	"net"
	"testing"
)

func TestSimplifyTimeoutError(t *testing.T) {
	oldErr := errors.New("Could not connect to  remote host 131.87.2.40: dial tcp 131.87.2.40:25: i/o timeout")
	newErr := simplifyError(oldErr)

	if newErr.Error() != "i/o timeout" {
		t.Fatal("invalid return value: ", newErr)
	}
}

func TestSimplifyStarttlsError(t *testing.T) {
	oldErr := errors.New("Conversation error with remote host 207.58.132.103:25: Bad return code for STARTTLS")
	newErr := simplifyError(oldErr)

	if newErr.Error() != "Bad return code for STARTTLS" {
		t.Fatal("invalid return value: ", newErr)
	}
}

func TestTimeout(t *testing.T) {
	target := zlib.GrabTarget{Addr: net.ParseIP("192.168.254.254")}
	result := NewMxHost(target)

	if result.Error.Error() != "i/o timeout" {
		t.Fatal("an unexpected error occured:", result.Error)
	}

	if result.HasStarttls() != nil {
		t.Fatal("host should not have starttls")
	}

}

func TestWithStarttls(t *testing.T) {
	target := zlib.GrabTarget{Addr: net.ParseIP("109.69.71.161")}
	result := NewMxHost(target)

	if *result.HasStarttls() != true {
		t.Fatal("host should have starttls")
	}

	if result.Error != nil {
		t.Fatal("an error occured: ", result.Error)
	}

	if len(result.ServerCertificate().DNSNames) == 0 {
		t.Fatal("DNSNames missing")
	}

	if result.ServerCertificateSHA1() == nil {
		t.Fatal("expected not nil")
	}
}

func TestWithoutStarttls(t *testing.T) {
	target := zlib.GrabTarget{Addr: net.ParseIP("198.23.62.105")}
	result := NewMxHost(target)

	if *result.HasStarttls() != false {
		t.Fatal("host should not have starttls")
	}

	if result.ServerCertificate() != nil {
		t.Fatal("nil expected")
	}

	if result.ServerCertificateSHA1() != nil {
		t.Fatal("nil expected")
	}

}
