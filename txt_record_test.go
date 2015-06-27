package main

import (
	"encoding/pem"
	"github.com/deckarep/golang-set"
	"github.com/zmap/zgrab/ztools/x509"
	"github.com/zmap/zgrab/ztools/ztls"
	"io/ioutil"
	"testing"
)

var (
	True  = true
	False = false
)

func TestTxtStarttls(t *testing.T) {
	check := func(expected bool, hosts []*MxHostSummary) {
		txt := createTxtRecord("", hosts)
		if txt.starttls != expected {
			t.Fatal("invalid starttls value:", txt.starttls, "for", hosts)
		}
	}

	// no hosts
	check(false, []*MxHostSummary{})

	// starttls == nil
	check(false, []*MxHostSummary{&MxHostSummary{}})

	// starttls == false
	check(false, []*MxHostSummary{&MxHostSummary{Starttls: &False}})

	// starttls == true
	check(true, []*MxHostSummary{&MxHostSummary{Starttls: &True}})

	// first:  starttls == true
	// second: unreachable
	check(true, []*MxHostSummary{&MxHostSummary{Starttls: &True}, &MxHostSummary{}})

	// first:  starttls == true
	// second: starttls == false
	check(false, []*MxHostSummary{&MxHostSummary{Starttls: &True}, &MxHostSummary{Starttls: &False}})
}

func TestTxtWithCertificate(t *testing.T) {
	cert := parseCertificate("testdata/example.com.crt")
	certs := []*x509.Certificate{cert}

	tlsVersions := mapset.NewThreadUnsafeSetFromSlice([]interface{}{string(ztls.TLSVersion(0x0303).Bytes())})
	tlsCiphers := mapset.NewThreadUnsafeSetFromSlice([]interface{}{string(ztls.TLSVersion(0xc02f).Bytes())})

	fingerprintA := [][]byte{[]byte("foo")}
	fingerprintB := [][]byte{[]byte("bar")}
	hostA := &MxHostSummary{Starttls: &True, tlsVersions: tlsVersions, tlsCipherSuites: tlsCiphers, fingerprints: fingerprintA, certificates: certs}
	hostB := &MxHostSummary{Starttls: &True, tlsVersions: tlsVersions, tlsCipherSuites: tlsCiphers, fingerprints: fingerprintA, certificates: certs}
	hostC := &MxHostSummary{Starttls: &True, tlsVersions: tlsVersions, tlsCipherSuites: tlsCiphers, fingerprints: fingerprintB, certificates: certs}

	txtRecord := createTxtRecord("", []*MxHostSummary{hostA, hostB, hostC})
	str := txtRecord.String()

	// no duplicate fingerprints should appear
	if str != "starttls=true updated=-62135596800 tls-versions=0303 tls-ciphers=c02f fingerprints=626172,666f6f certificate-problems=mismatch" {
		t.Fatal("invalid string:", str)
	}
}

func parseCertificate(name string) *x509.Certificate {
	fileBytes, _ := ioutil.ReadFile(name)
	p, _ := pem.Decode(fileBytes)
	cert, _ := x509.ParseCertificate(p.Bytes)
	return cert
}
