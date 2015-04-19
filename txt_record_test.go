package main

import (
	"testing"
)

var (
	True  = true
	False = false
)

func TestTxtStarttls(t *testing.T) {
	check := func(expected bool, hosts []MxHost) {
		txt := createTxtRecord("", hosts)
		if txt.starttls != expected {
			t.Fatal("invalid starttls value:", txt.starttls, "for", hosts)
		}
	}

	// no hosts
	check(false, []MxHost{})

	// starttls == nil
	check(false, []MxHost{MxHost{}})

	// starttls == false
	check(false, []MxHost{MxHost{starttls: &False}})

	// starttls == true
	check(true, []MxHost{MxHost{starttls: &True}})

	// first:  starttls == true
	// second: unreachable
	check(true, []MxHost{MxHost{starttls: &True}, MxHost{}})

	// first:  starttls == true
	// second: starttls == false
	check(false, []MxHost{MxHost{starttls: &True}, MxHost{starttls: &False}})
}

func TestTxtWithCertificate(t *testing.T) {
	fingerprintA := []byte("deadbeef")
	fingerprintB := []byte("foobar")
	hostA := MxHost{starttls: &True, serverFingerprint: &fingerprintA}
	hostB := MxHost{starttls: &True, serverFingerprint: &fingerprintA}
	hostC := MxHost{starttls: &True, serverFingerprint: &fingerprintB}

	txtRecord := createTxtRecord("", []MxHost{hostA, hostB, hostC})
	str := txtRecord.String()

	// no duplicate fingerprints should appear
	if str != "starttls=true fingerprints=deadbeef,foobar" {
		t.Fatal("invalid string:", str)
	}
}
