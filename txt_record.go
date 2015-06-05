package main

import (
	"bytes"
	"github.com/deckarep/golang-set"
	"strconv"
)

type TxtRecord struct {
	domain       string
	starttls     bool       // true if all reachable hosts have starttls
	fingerprints mapset.Set // the union of all hosts
	certProblems mapset.Set // the union of all hosts
	tlsVersions  mapset.Set // the intersection of all hosts
	tlsCiphers   mapset.Set // the intersection of all hosts
	trusted      mapset.Set // the intersection of all hosts: list of root stores with a valid chain
	updatedAt    int64
}

// Creates a TxtRecord from one or more MxHostSummary objects
func createTxtRecord(hostname string, hosts []*MxHostSummary) (record TxtRecord) {
	record.domain = hostname

	// Check if tls handshake to alle hosts have succeeded
	tlsFound := false
	for i, host := range hosts {
		// Update Timestamp
		updatedAt := host.Updated.Unix()
		if i == 0 || updatedAt > record.updatedAt {
			record.updatedAt = updatedAt
		}

		if host.Starttls != nil {
			if !tlsFound {
				// set initial value
				tlsFound = true
				record.starttls = *host.Starttls
			}
			// It MAY happen that STARTTLS is true but the TLS handshake
			// is not successful because certificates can not be parsed.
			if *host.Starttls == false || len(host.certificates) == 0 {
				record.starttls = false
			}
		}
	}
	if !record.starttls {
		// no sense to go further
		return
	}

	record.fingerprints = mapset.NewThreadUnsafeSet()
	record.certProblems = mapset.NewThreadUnsafeSet()
	record.trusted = mapset.NewThreadUnsafeSet()

	for _, host := range hosts {
		if host.tlsVersions != nil {
			validity := host.validity

			if record.tlsVersions == nil {
				// Just copy, it's the first one
				record.tlsVersions = host.tlsVersions
				record.tlsCiphers = host.tlsCipherSuites
				record.trusted = validity.TrustedNames()
			} else {
				// Calculate the intersection
				record.tlsVersions = record.tlsVersions.Intersect(host.tlsVersions)
				record.tlsCiphers = record.tlsCiphers.Intersect(host.tlsCipherSuites)
				record.trusted = record.trusted.Intersect(validity.TrustedNames())
			}

			// Has the server certificate been parsed successfully?
			if fingerprint := host.ServerFingerprint(); fingerprint != nil {
				record.fingerprints.Add(string(*fingerprint))

				if !host.CertificateValidForDomain(hostname) {
					record.certProblems.Add("mismatch")
				}
				if validity.Expired {
					record.certProblems.Add("expired")
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

	if record.tlsVersions != nil {
		if record.tlsVersions.Cardinality() > 0 {
			addValue("tls-versions", joinSet(record.tlsVersions, true))
		}

		if record.tlsCiphers.Cardinality() > 0 {
			addValue("tls-ciphers", joinSet(record.tlsCiphers, true))
		}

		if record.fingerprints.Cardinality() > 0 {
			addValue("fingerprints", joinSet(record.fingerprints, true))
		}

		if record.trusted.Cardinality() > 0 {
			addValue("trusted", joinSet(record.trusted, false))
		}

		if record.certProblems.Cardinality() > 0 {
			addValue("certificate-problems", joinSet(record.certProblems, false))
		}
	}

	return buffer.String()
}
