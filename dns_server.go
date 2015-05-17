package main

import (
	"github.com/miekg/dns"
	"log"
)

type DnsServer struct {
	server dns.Server
}

func NewDnsServer() *DnsServer {
	server := &DnsServer{
		server: dns.Server{
			Addr: ":5533",
			Net:  "udp",
		},
	}
	server.server.Handler = dns.HandlerFunc(server.handle)

	// Serve in the background
	go func() {
		err := server.server.ListenAndServe()
		if err != nil {
			log.Panicf("Failed to setup the server: %s\n", err.Error())
		}
	}()

	return server
}

// Shuts down the server
func (dnsServer *DnsServer) Close() error {
	return dnsServer.server.Shutdown()
}

// Handles a DNS message
func (dnsServer *DnsServer) handle(w dns.ResponseWriter, r *dns.Msg) {
	var response *string
	msg := new(dns.Msg)
	name := r.Question[0].Name
	domainLen := len(name) - len(dnsZone)
	if len(dnsZone) > 1 {
		domainLen -= 1
	}

	if domainLen > 0 {
		// cut off the zone
		response = mxProcessor.GetValue(name[:domainLen])
	}

	if response != nil {
		// Build reply
		msg.SetReply(r)
		t := new(dns.TXT)
		t.Hdr = dns.RR_Header{
			Name:   r.Question[0].Name,
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    600,
		}
		t.Txt = SplitByLength(*response, dnsMaxItemLength)

		msg.Answer = append(msg.Answer, t)
	} else {
		// No answer available
		msg.SetRcode(r, dns.RcodeServerFailure)
	}

	w.WriteMsg(msg)
}
