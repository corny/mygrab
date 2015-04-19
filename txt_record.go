package main

import (
	"bytes"
)

type TxtRecord struct {
	starttls     bool
	fingerprints []string
}

// Creates a TxtRecord from one or more MxHost objects
func createTxtRecord(hostname string, hosts []MxHost) (record TxtRecord) {

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
		// no sense do go further
		return
	}

	for _, host := range hosts {
		if host.serverFingerprint != nil {
			record.fingerprints = append(record.fingerprints, string(*host.serverFingerprint))
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

	if len(record.fingerprints) > 0 {
		buffer.WriteString(" fingerprints=")
		for i, fingerprint := range record.fingerprints {
			if i > 0 {
				buffer.WriteByte(',')
			}
			buffer.WriteString(fingerprint)

		}
	}

	return buffer.String()
}
