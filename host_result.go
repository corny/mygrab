package main

import (
	"errors"
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/ztls"
	"net"
	"strings"
)

// Encapsulates the zlib.Grab struct
type HostResult struct {
	grab         *zlib.Grab
	TlsHandshake *ztls.ServerHandshake
	StartTLS     *zlib.StartTLSEvent
	connect      *zlib.ConnectEvent
	MailBanner   string
	Error        error
}

// Host() delegates to grab.Host()
func (result *HostResult) Host() net.IP {
	return result.grab.Host
}

func (result *HostResult) HasStarttls() bool {
	return result.StartTLS != nil && result.TlsHandshake != nil
}

func simplfiyError(err error) error {
	msg := err.Error()
	if strings.HasPrefix(msg, "Conversation error") || strings.HasPrefix(msg, "Could not connect") || strings.HasPrefix(msg, "dial tcp") {
		if i := strings.LastIndex(msg, ": "); i != -1 {
			return errors.New(msg[i+2 : len(msg)])
		}
	}
	return err
}

func NewZgrabResult(target zlib.GrabTarget) *HostResult {
	result := &HostResult{grab: zlib.GrabBanner(zlibConfig, &target)}

	for _, entry := range result.grab.Log {
		data := entry.Data

		switch data := data.(type) {
		case *zlib.TLSHandshakeEvent:
			result.TlsHandshake = data.GetHandshakeLog()
		case *zlib.StartTLSEvent:
			result.StartTLS = data
		case *zlib.MailBannerEvent:
			result.MailBanner = data.Banner
		}

		if entry.Error != nil {
			result.Error = simplfiyError(entry.Error)
		}
	}

	return result
}
