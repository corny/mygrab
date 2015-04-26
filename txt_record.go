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
		if host.UpdatedAt != nil {
			updatedAt := host.UpdatedAt.Unix()
			if i == 0 || updatedAt > record.updatedAt {
				record.updatedAt = updatedAt
			}
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

	addValue := func(key string, value string) {
		if buffer.Len() > 0 {
			buffer.WriteString(" ")
		}
		buffer.WriteString(key)
		buffer.WriteString("=")
		buffer.WriteString(value)
	}

	if record.starttls {
		addValue("starttls", "true")
	} else {
		addValue("starttls", "false")
	}

	if !record.starttls {
		return buffer.String()
	}

	addValue("updated", strconv.FormatInt(record.updatedAt, 10))

	// Only one TLS version?
	if record.tlsVersions.Len() == 1 {
		addValue("tls-version", record.tlsVersions.String())
	}

	if record.certErrors.Len() > 0 {
		addValue("certificate-errors", record.certErrors.String())
	}

	if len(record.fingerprints) > 0 {
		addValue("fingerprint", record.String())
	}

	return buffer.String()
}
