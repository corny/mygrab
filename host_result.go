package main

import (
	"errors"
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/x509"
	"github.com/zmap/zgrab/ztools/ztls"
	"net"
	"strings"
)

// Encapsulates the zlib.Grab struct
type HostResult struct {
	grab         *zlib.Grab
	TLSHandshake *ztls.ServerHandshake
	StartTLS     *zlib.StartTLSEvent
	connect      *zlib.ConnectEvent
	MailBanner   string
	Error        error
}

// Host() delegates to grab.Host()
func (result *HostResult) Host() net.IP {
	return result.grab.Host
}

// The received certificates
func (result *HostResult) Certificates() []*x509.Certificate {
	if result.TLSHandshake == nil || result.TLSHandshake.ServerCertificates == nil {
		return nil
	} else {
		return result.TLSHandshake.ServerCertificates.ParsedCertificates
	}
}

// Host() delegates to grab.Host()
func (result *HostResult) ServerCertificate() *x509.Certificate {
	certs := result.Certificates()
	if len(certs) == 0 {
		return nil
	} else {
		return certs[0]
	}
}

// Host() delegates to grab.Host()
func (result *HostResult) ServerCertificateSHA1() *string {
	cert := result.ServerCertificate()
	if cert == nil {
		return nil
	}
	str := string(cert.FingerprintSHA1)
	return &str
}

// The pointer to error message or nil
func (result *HostResult) ErrorString() *string {
	if result.Error == nil {
		return nil
	}
	str := result.Error.Error()
	return &str

}

// Returns nil pointer if no STARTTLS entry exists
// Otherwise it returns a pointer to bool.
func (result *HostResult) HasStarttls() *bool {
	if result.StartTLS == nil {
		return nil
	}
	boolean := result.TLSHandshake != nil
	return &boolean
}

func (result *HostResult) TLSHello() *ztls.ServerHello {
	if result.TLSHandshake == nil || result.TLSHandshake.ServerHello == nil {
		return nil
	}
	return result.TLSHandshake.ServerHello
}

func (result *HostResult) TLSCipherSuite() *string {
	hello := result.TLSHello()
	if hello == nil {
		return nil
	}

	str := hello.CipherSuite.String()
	return &str
}

func (result *HostResult) TLSVersion() *string {
	hello := result.TLSHello()
	if hello == nil {
		return nil
	}

	str := hello.Version.String()
	return &str
}

var stripErrors = []string{"Conversation error", "Could not connect", "dial tcp", "read tcp"}

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
func NewHostResult(target zlib.GrabTarget) HostResult {
	result := HostResult{grab: zlib.GrabBanner(zlibConfig, &target)}

	for _, entry := range result.grab.Log {
		data := entry.Data

		switch data := data.(type) {
		case *zlib.TLSHandshakeEvent:
			result.TLSHandshake = data.GetHandshakeLog()
		case *zlib.StartTLSEvent:
			result.StartTLS = data
		case *zlib.MailBannerEvent:
			result.MailBanner = data.Banner
		}

		if entry.Error != nil {
			result.Error = simplifyError(entry.Error)
		}
	}

	return result
}
