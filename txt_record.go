package main

import (
	"bytes"
	"encoding/hex"
)

type TxtRecord struct {
	starttls     bool
	fingerprints *stringSet
	tlsVersions  *stringSet
}

// Creates a TxtRecord from one or more MxHost objects
func createTxtRecord(hostname string, hosts []*MxHost) (record TxtRecord) {

	// Check if all reachable hosts has StartTLS
	starttlsFound := false
	for _, host := range hosts {
		if host.starttls != nil {
			if !starttlsFound {
				// set initial value
				record.starttls = true
				starttlsFound = true
			}
			if *host.starttls == false {
				record.starttls = false
			}
		}
	}
	if !record.starttls {
		// no sense to go further
		return
	}

	record.tlsVersions = &stringSet{}
	record.fingerprints = &stringSet{}

	for _, host := range hosts {
		if host.tlsVersion != nil {
			record.tlsVersions.Add(string(*host.tlsVersion))
		}
		if host.serverFingerprint != nil {
			record.fingerprints.Add(hex.EncodeToString(*host.serverFingerprint))
		}
	}

	return
}

// String representation
func (record *TxtRecord) String() string {
	if !record.starttls {
		return "starttls=false"
	}

	buffer := new(bytes.Buffer)
	buffer.WriteString("starttls=true")

	if record.tlsVersions.Len() == 1 {
		buffer.WriteString(" tls-version=")
		buffer.WriteString(record.tlsVersions.String())
	}

	if len(*record.fingerprints) > 0 {
		buffer.WriteString(" fingerprints=")
		buffer.WriteString(record.fingerprints.String())
	}

	return buffer.String()
}
