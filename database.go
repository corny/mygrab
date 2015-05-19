package main

import (
	"crypto/rsa"
	"database/sql"
	"encoding/hex"
	"github.com/hashicorp/golang-lru"
	_ "github.com/lib/pq"
	"github.com/zmap/zgrab/ztools/x509"
	"log"
	"strings"
)

var (
	dbconn     *sql.DB
	knownCerts *lru.Cache // For performance boosting
)

// Establish the database connection
func connect(dataSourceName string) {
	var err error

	// Initialize databse connection
	if dbconn, err = sql.Open("postgres", dataSourceName); err != nil {
		panic(err)
	}

	dbconn.SetMaxOpenConns(10)

	// Initialize cache
	if knownCerts, err = lru.New(1024); err != nil {
		panic(err)
	}

}

// Reads mx_hosts from the domains table and passes them to the MxProcessor
func resolveDomainMxHosts() {
	log.Println("load mx_hosts from domains")
	rows, err := dbconn.Query("SELECT DISTINCT unnest(mx_hosts) FROM domains")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var hostname string
		if err := rows.Scan(&hostname); err != nil {
			log.Fatal(err)
		}
		mxProcessor.NewJob(hostname)
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

}

func saveDomain(job *DnsJob) {
	result := job.Result
	domain := job.Query.Domain
	params := []interface{}{StringArray(result.Results), result.Secure, result.ErrorMessage(), result.WhyBogus, domain}

	var id int
	err := dbconn.QueryRow("SELECT 1 FROM domains WHERE name = $1", domain).Scan(&id)
	switch err {
	case sql.ErrNoRows:
		// not yet present
		_, err = dbconn.Exec("INSERT INTO domains (mx_hosts, dns_secure, dns_error, dns_bogus, name) VALUES ($1,$2,$3,$4,$5)", params...)
		if err != nil {
			log.Panicln(err)
		}
	case nil:
		_, err = dbconn.Exec("UPDATE domains SET mx_hosts=$1, dns_secure=$2, dns_error=$3, dns_bogus=$4 WHERE name=$5", params...)
		if err != nil {
			log.Panicln(err)
		}
	default:
		log.Fatal(err)
	}
}

func saveMxAddresses(job *DnsJob) {
	hostname := job.Query.Domain

	tx, err := dbconn.Begin()
	if err != nil {
		log.Fatal(err)
	}

	family := 0
	if job.Query.Type == TypeA {
		family = 4
	} else {
		family = 6
	}

	_, err = tx.Exec("DELETE FROM mx_records WHERE hostname=$1 AND family(address)=$2", hostname, family)
	if err != nil {
		log.Fatal(err)
	}

	for _, address := range UniqueStrings(job.Results()) {
		_, err = tx.Exec("INSERT INTO mx_records (hostname, address, dns_secure, dns_error, dns_bogus) VALUES ($1,$2,$3,$4,$5)", hostname, address, false, nil, nil) // result.Secure, result.ErrorMessage(), "result.WhyBogus")

		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") {
				// Just a race condition
				log.Println("duplicate key for", hostname, address)
			} else {
				log.Fatal(err)
			}
			tx.Rollback()
			return
		}
	}

	if err = tx.Commit(); err != nil {
		log.Fatal(err)
	}
}

// Saves a certificate if it is not saved yet
func saveCertificate(cert *x509.Certificate) {
	sha1sum := string(cert.FingerprintSHA1)
	sha1hex := hex.EncodeToString(cert.FingerprintSHA1)

	// Certificate cached?
	if _, ok := knownCerts.Get(sha1sum); ok {
		return
	}

	var exists bool
	err := dbconn.QueryRow("SELECT TRUE FROM raw_certificates WHERE id = $1", sha1sum).Scan(&exists)
	switch err {
	case sql.ErrNoRows:
		// not yet present
		if _, err = dbconn.Exec("INSERT INTO raw_certificates (id, raw) VALUES ($1,$2)", sha1sum, cert.Raw); err != nil {
			if strings.Contains(err.Error(), "duplicate key") {
				// Just a race condition
				log.Println("duplicate key", sha1hex)
				return
			} else {
				log.Panic(err, sha1hex)
			}
		}

		subject := string(x509.SHA1Fingerprint(cert.RawSubject))
		issuer := string(x509.SHA1Fingerprint(cert.RawIssuer))
		pubkey := string(x509.SHA1Fingerprint(cert.RawSubjectPublicKeyInfo))
		signatureAlgorithm := cert.SignatureAlgorithmName()
		publicKeyAlgorithm := cert.PublicKeyAlgorithmName()
		selfSigned := cert.Subject.CommonName == cert.Issuer.CommonName

		// Key length
		var pubkeySize *int
		switch key := cert.PublicKey.(type) {
		case *rsa.PublicKey:
			len := key.N.BitLen()
			pubkeySize = &len
		}

		_, err = dbconn.Exec("INSERT INTO certificates (id, subject_id, issuer_id, key_id, key_size, signature_algorithm, key_algorithm, is_self_signed, is_ca, first_seen_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, NOW())",
			sha1sum, subject, issuer, pubkey, pubkeySize, signatureAlgorithm, publicKeyAlgorithm, selfSigned, cert.IsCA)
		if err != nil {
			log.Panicln(err, sha1hex)
		}
		knownCerts.Add(sha1sum, 1)
	case nil:
		// already present
		knownCerts.Add(sha1sum, 1)
	default:
		log.Fatal(err)
	}

}

// Saves a MxHost in the database
func saveMxHostSummary(result *MxHostSummary) {
	address := result.address.String()

	var id int
	err := dbconn.QueryRow("SELECT id FROM mx_hosts WHERE address = $1", address).Scan(&id)

	var rootFingerprint *[]byte
	var certTrusted *bool
	var certExpired *bool
	var certError *string

	// certificate validity
	if v := result.validity; v != nil {
		var trusted bool
		if root := v.RootCertificate(); root != nil {
			trusted = true
			fingerprint := []byte(root.FingerprintSHA1)
			rootFingerprint = &fingerprint
		} else {
			trusted = false
		}
		certTrusted = &trusted

		// expiriation status of the server certificate
		certExpired = &v.Expired

		// Copy validation error
		certError = v.ErrorString()
	}

	params := []interface{}{
		result.Error,
		result.Starttls,
		ByteaArray(setToByteArrays(result.tlsVersions)),
		ByteaArray(setToByteArrays(result.tlsCipherSuites)),
		result.ServerFingerprint(),
		ByteaArray(result.CaFingerprints()),
		rootFingerprint,
		certExpired,
		certTrusted,
		certError,
		result.ecdheCurveType,
		result.ecdheCurveId,
		result.Updated,
		address,
	}

	switch err {
	case sql.ErrNoRows:
		// not yet present
		_, err := dbconn.Exec("INSERT INTO mx_hosts (error, starttls, tls_versions, tls_cipher_suites, certificate_id, ca_certificate_ids, root_certificate_id, cert_expired, cert_trusted, cert_error, ecdhe_curve_type, ecdhe_curve_id, updated_at, address) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)", params...)
		if err != nil {
			log.Panicln(err)
		}
	case nil:
		_, err := dbconn.Exec("UPDATE mx_hosts SET error=$1, starttls=$2, tls_versions=$3, tls_cipher_suites=$4, certificate_id=$5, ca_certificate_ids=$6, root_certificate_id=$7, cert_expired=$8, cert_trusted=$9, cert_error=$10, ecdhe_curve_type=$11, ecdhe_curve_id=$12, updated_at=$13 WHERE address = $14", params...)
		if err != nil {
			log.Panicln(err)
		}
	default:
		log.Fatal(err)
	}
}

// Saves a MxDomain in the database
func saveMxDomain(record *TxtRecord) {
	txt := record.String()

	var id int
	err := dbconn.QueryRow("SELECT id FROM mx_domains WHERE name = $1", record.domain).Scan(&id)

	switch err {
	case sql.ErrNoRows:
		// not yet present
		_, err := dbconn.Exec("INSERT INTO mx_domains (name,txt) VALUES ($1,$2)", record.domain, txt)
		if err != nil {
			log.Panicln(err)
		}
	case nil:
		_, err := dbconn.Exec("UPDATE mx_domains SET txt=$1 WHERE id = $2", txt, id)
		if err != nil {
			log.Panicln(err)
		}
	default:
		log.Fatal(err)
	}
}
