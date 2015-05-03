package main

import (
	"errors"
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/x509"
	"github.com/zmap/zgrab/ztools/ztls"
	"net"
	"strings"
	"time"
)

// Encapsulates the zlib.Grab struct
type MxHost struct {
	address           net.IP
	grab              *zlib.Grab
	TLSHandshake      *ztls.ServerHandshake
	starttls          *bool
	serverFingerprint *[]byte
	caFingerprints    [][]byte
	tlsVersion        *string
	tlsCipherSuite    *string
	connect           *zlib.ConnectEvent
	Error             *string
	UpdatedAt         *time.Time
}

// The received certificates
func (result *MxHost) Certificates() []*x509.Certificate {
	if result.TLSHandshake == nil || result.TLSHandshake.ServerCertificates == nil {
		return nil
	} else {
		return result.TLSHandshake.ServerCertificates.ParsedCertificates
	}
}

// Host() delegates to grab.Host()
func (result *MxHost) ServerCertificate() *x509.Certificate {
	certs := result.Certificates()
	if len(certs) == 0 {
		return nil
	} else {
		return certs[0]
	}
}

func (result *MxHost) TLSHello() *ztls.ServerHello {
	if result.TLSHandshake == nil || result.TLSHandshake.ServerHello == nil {
		return nil
	}
	return result.TLSHandshake.ServerHello
}

// Checks if the certificate is not yet valid or expired
func (result *MxHost) certificateExpired() *bool {
	c := result.ServerCertificate()
	if c == nil {
		return nil
	}
	now := time.Now()
	val := now.Before(c.NotBefore) || now.After(c.NotAfter)
	return &val
}

// Checks if the certificate is valid for a given domain name
func (result *MxHost) certificateValidForDomain(domain string) bool {
	return result.ServerCertificate().VerifyHostname(domain) == nil
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

// Creates a ZgrabResult
func NewMxHost(address net.IP) MxHost {
	result := MxHost{address: address}
	result.grab = zlib.GrabBanner(zlibConfig, &zlib.GrabTarget{Addr: address})
	now := time.Now()
	result.UpdatedAt = &now

	// Loop over the banner log
	for _, entry := range result.grab.Log {
		data := entry.Data

		switch data := data.(type) {
		case *zlib.TLSHandshakeEvent:
			result.TLSHandshake = data.GetHandshakeLog()
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
	hello := result.TLSHello()
	if hello != nil {
		cipherSuite := hello.CipherSuite.String()
		version := hello.Version.String()
		result.tlsVersion = &version
		result.tlsCipherSuite = &cipherSuite
	}

	// Copy SHA1 Fingerprints
	certs := result.Certificates()
	if certs != nil {
		result.caFingerprints = make([][]byte, len(certs)-1)
		for i, cert := range certs {
			sha1 := []byte(cert.FingerprintSHA1)
			if i == 0 {
				result.serverFingerprint = &sha1
			} else {
				result.caFingerprints[i-1] = sha1
			}
		}
	}

	return result
}
