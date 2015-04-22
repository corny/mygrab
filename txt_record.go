package main

import (
	"bytes"
	"encoding/hex"
	"strconv"
)

type TxtRecord struct {
	domain       string
	starttls     bool
	fingerprints stringSet
	tlsVersions  stringSet
	certErrors   stringSet
	updatedAt    int64
}

// Creates a TxtRecord from one or more MxHost objects
func createTxtRecord(hostname string, hosts []*MxHost) (record TxtRecord) {
	record.domain = hostname

	// Check if all reachable hosts has StartTLS
	starttlsFound := false
	for i, host := range hosts {
		// Update Timestamp
		updatedAt := host.UpdatedAt.Unix()
		if i == 0 || updatedAt > record.updatedAt {
			record.updatedAt = updatedAt
		}

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

	record.tlsVersions = stringSet{}
	record.fingerprints = stringSet{}
	record.certErrors = stringSet{}

	for _, host := range hosts {
		if host.tlsVersion != nil {
			record.tlsVersions.Add(string(*host.tlsVersion))

			// Has the server certificate been successfully parsed?
			if host.serverFingerprint != nil {
				record.fingerprints.Add(hex.EncodeToString(*host.serverFingerprint))

				if !host.certificateValidForDomain(hostname) {
					record.certErrors.Add("mismatch")
				}
				if *host.certificateExpired() {
					record.certErrors.Add("expired")
				}
			}
		}
	}

	return
}

// String representation
func (record *TxtRecord) String() string {
	buffer := new(bytes.Buffer)
	if record.starttls {
		buffer.WriteString("starttls=true")
	} else {
		buffer.WriteString("starttls=false")
	}

	if !record.starttls {
		return buffer.String()
	}

	buffer.WriteString(" updated=")
	buffer.WriteString(strconv.FormatInt(record.updatedAt, 10))

	// Only one TLS version?
	if record.tlsVersions.Len() == 1 {
		buffer.WriteString(" tls-version=")
		buffer.WriteString(record.tlsVersions.String())
	}

	if record.fingerprints.Len() > 0 {
		buffer.WriteString(" fingerprint=")
		buffer.WriteString(record.fingerprints.String())

		if record.certErrors.Len() > 0 {
			buffer.WriteString(" certificate-errors=")
			buffer.WriteString(record.certErrors.String())
		}
	}

	return buffer.String()
}
