package main

import (
	"errors"
	"github.com/zmap/zgrab/ztools/ztls"
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
	address := net.ParseIP("192.168.254.254")
	result := NewMxHostGrab(address, ztls.VersionTLS12)

	if *result.Error != "i/o timeout" {
		t.Fatal("an unexpected error occured:", result.Error)
	}

	if result.starttls != nil {
		t.Fatal("host should not have starttls")
	}

}

func TestWithStarttls(t *testing.T) {
	address := net.ParseIP("109.69.71.161")
	result := NewMxHostGrab(address, ztls.VersionTLS12)

	if *result.starttls != true {
		t.Fatal("host should have starttls")
	}

	if result.Error != nil {
		t.Fatal("an error occured: ", result.Error)
	}

	if len(result.certificates) == 0 {
		t.Fatal("no certificates found")
	}

	if len(result.certificates[0].DNSNames) == 0 {
		t.Fatal("DNSNames missing")
	}
}

func TestWithoutStarttls(t *testing.T) {
	address := net.ParseIP("198.23.62.105")
	result := NewMxHostGrab(address, ztls.VersionTLS12)

	if *result.starttls != false {
		t.Fatal("host should not have starttls")
	}

	if result.certificates != nil {
		t.Fatal("nil expected")
	}

}
