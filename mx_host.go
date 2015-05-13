package main

import (
	"errors"
	"github.com/deckarep/golang-set"
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/x509"
	"github.com/zmap/zgrab/ztools/ztls"
	"net"
	"strings"
	"time"
)

// Summarizes the results of multiple connections to a single host
type MxHostSummary struct {
	address         net.IP
	UpdatedAt       time.Time
	starttls        *bool
	tlsVersions     mapset.Set
	tlsCipherSuites mapset.Set
	certificates    []*x509.Certificate
	fingerprints    [][]byte
	Error           *string // only the first error
}

// The result of a single connection attempt using zlib.Grab
type MxHostGrab struct {
	starttls       *bool
	tlsVersion     ztls.TLSVersion
	tlsCipherSuite ztls.CipherSuite
	certificates   []*x509.Certificate
	Error          *string
}

// Summry of multiple connection attemps to a single host
func NewMxHostSummary(address net.IP) MxHostSummary {
	result := MxHostSummary{
		address:   address,
		UpdatedAt: time.Now(),
	}

	// The first connection attempt with up to TLS 1.2
	grab := NewMxHostGrab(address, ztls.VersionTLS12)

	result.starttls = grab.starttls
	result.Error = grab.Error

	// Was the TLS handshake successful?
	if result.starttls != nil && *result.starttls {
		result.tlsVersions = mapset.NewThreadUnsafeSet()
		result.tlsCipherSuites = mapset.NewThreadUnsafeSet()
		result.Append(grab)

		// Try TLS 1.0 as well if we had a TLS 1.2 handshake
		if grab.tlsVersion == ztls.VersionTLS12 {
			if grab = NewMxHostGrab(address, ztls.VersionTLS10); grab.TLSSuccessful() {
				result.Append(grab)
			}

		}

	}

	// calculate fingerprints
	if result.certificates != nil {
		result.fingerprints = result.Fingerprints()
	}

	return result
}

// Result of a single connection attempt with ZGrab
func NewMxHostGrab(address net.IP, tlsVersion uint16) *MxHostGrab {
	result := &MxHostGrab{}
	var tlsHandshake *ztls.ServerHandshake
	var tlsHello *ztls.ServerHello

	// Create a local copy of the default config
	config := *zlibConfig
	config.TLSVersion = tlsVersion // maximum TLS version

	// Grab the banner
	banner := zlib.GrabBanner(&config, &zlib.GrabTarget{Addr: address})

	// Loop trough the banner log
	for _, entry := range banner.Log {
		data := entry.Data

		switch data := data.(type) {
		case *zlib.TLSHandshakeEvent:
			tlsHandshake = data.GetHandshakeLog()
			tlsHello = tlsHandshake.ServerHello
		case *zlib.StartTLSEvent:
			val := entry.Error == nil
			result.starttls = &val
		}

		if entry.Error != nil {
			// If an error occurs we expect the log entry to be the last
			err := simplifyError(entry.Error).Error()
			result.Error = &err
		}
	}

	// Copy TLS Parameters
	if tlsHello != nil {
		result.tlsVersion = tlsHello.Version
		result.tlsCipherSuite = tlsHello.CipherSuite
	}

	// Copy Certificates
	if tlsHandshake != nil {
		result.certificates = tlsHandshake.ServerCertificates.ParsedCertificates
	}

	return result
}

// The received certificates
func (summary *MxHostSummary) Fingerprints() [][]byte {

	fingerprints := make([][]byte, len(summary.certificates))
	for i, cert := range summary.certificates {
		fingerprints[i] = []byte(cert.FingerprintSHA1)
	}

	return fingerprints
}

// Checks if the certificate is not yet valid or expired
func (summary *MxHostSummary) CertificateExpired() *bool {
	if summary.certificates == nil {
		return nil
	}
	cert := summary.certificates[0]
	now := time.Now()
	val := now.Before(cert.NotBefore) || now.After(cert.NotAfter)
	return &val
}

func (summary *MxHostSummary) ServerFingerprint() *[]byte {
	if summary.fingerprints == nil {
		return nil
	}
	return &summary.fingerprints[0]
}

func (summary *MxHostSummary) CaFingerprints() [][]byte {
	if summary.fingerprints == nil {
		return nil
	}
	fingerprints := make([][]byte, len(summary.fingerprints)-1)
	for i, fingerprint := range summary.fingerprints {
		if i > 0 {
			fingerprints[i-1] = fingerprint
		}
	}
	return fingerprints
}

// Appends a MxHostGrab to the MxHostSummary
func (summary *MxHostSummary) Append(grab *MxHostGrab) {
	if summary.fingerprints == nil {
		summary.certificates = grab.certificates
	}
	summary.tlsCipherSuites.Add(string(grab.tlsCipherSuite.Bytes()))
	summary.tlsVersions.Add(string(grab.tlsVersion.Bytes()))
}

// Checks if the certificate is valid for a given domain name
func (summary *MxHostSummary) CertificateValidForDomain(domain string) bool {
	return summary.certificates[0].VerifyHostname(domain) == nil
}

// Was the TLS Handshake successful?
func (result *MxHostGrab) TLSSuccessful() bool {
	return result.certificates != nil
}

var stripErrors = []string{
	"Conversation error",
	"Could not connect",
	"dial tcp",
	"read tcp",
	"write tcp",
}

func simplifyError(err error) error {
	msg := err.Error()
	for _, prefix := range stripErrors {
		if strings.HasPrefix(msg, prefix) {
			if i := strings.LastIndex(msg, ": "); i != -1 {
				return errors.New(msg[i+2 : len(msg)])
			}
		}
	}
	return err
}
