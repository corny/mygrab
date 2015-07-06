package main

import (
	"github.com/miekg/dns"
	"log"
	"strings"
)

type DnsServer struct {
	server dns.Server
	zone   string
}

// Creates a new DNS server
func NewDnsServer(address string, zone string) *DnsServer {
	// Append dot to zone if missing
	if !strings.HasSuffix(zone, ".") {
		zone = zone + "."
	}

	log.Println("Starting DNS server with zone " + zone)

	// Prepend dot to zone if missing
	if !strings.HasPrefix(zone, ".") {
		zone = "." + zone
	}

	server := &DnsServer{
		zone: zone,
		server: dns.Server{
			Addr: address,
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
	name := r.Question[0].Name

	// Wrong zone?
	if !strings.HasSuffix(name, dnsServer.zone) {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(m)
		return
	}

	// Cut off the zone and retrieve the TXT record
	response := mxProcessor.GetValue(name[:len(name)-len(dnsServer.zone)])

	if response == nil {
		// Do not send any answers at this place.
		// The client should resend its message and
		// then we will hopefully have the answer.
		return
	}

	// Build the reply
	t := new(dns.TXT)
	t.Hdr = dns.RR_Header{
		Name:   r.Question[0].Name,
		Rrtype: dns.TypeTXT,
		Class:  dns.ClassINET,
		Ttl:    600,
	}
	t.Txt = SplitByLength(*response, dnsMaxItemLength)

	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Answer = []dns.RR{t}
	w.WriteMsg(msg)
}
