package main

import (
	"testing"
)

func TestBlacklisted(t *testing.T) {
	blacklist := NewOpensslBlacklist()

	// should not blacklisted
	cert := parseCertificate("/usr/share/doc/openssl-blacklist/examples/good_x509.pem")
	if blacklist.Contains(cert) {
		t.Fatal("blacklisted")
	}

	// should be blacklisted
	cert = parseCertificate("/usr/share/doc/openssl-blacklist/examples/bad_x509.pem")
	if !blacklist.Contains(cert) {
		t.Fatal("not blacklisted")
	}
}
