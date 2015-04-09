package main

import (
	"database/sql"
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

func saveDomain(domain string, result *DnsResult) {

	params := []interface{}{StringArray(result.Results), result.Secure, result.Error, result.WhyBogus, domain}

	var id int
	err := dbconn.QueryRow("SELECT 1 FROM domains WHERE name = $1", domain).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		// not yet present
		_, err = dbconn.Exec("INSERT INTO domains (mx_hosts, dns_secure, dns_error, dns_bogus, name) VALUES ($1,$2,$3,$4,$5)", params...)
		if err != nil {
			log.Panicln(err)
		}
	case err != nil:
		log.Fatal(err)
	default:
		_, err = dbconn.Exec("UPDATE domains SET mx_hosts=$1, dns_secure=$2, dns_error=$3, dns_bogus=$4 WHERE name=$5", params...)
		if err != nil {
			log.Panicln(err)
		}
	}
}

func saveMxRecords(hostnames []string, result []*DnsJob) {
	// TODO better error handling

	tx, err := dbconn.Begin()
	log.Println(err)
	_, err = tx.Exec("DELETE FROM mx_records WHERE hostname IN ($1)", strings.Join(hostnames, ","))

	for _, job := range result {
		for _, address := range job.Results() {
			_, err = tx.Exec("INSERT INTO mx_records (hostname, address, dns_secure, dns_error, dns_bogus) VALUES ($1,$2,$3,$4,$5)", job.Query.Domain, address, job.Result.Secure, job.Result.Error, job.Result.WhyBogus)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	err = tx.Commit()
	log.Println(err)
}

// Saves a certificate if it is not saved yet
func saveCertificate(cert *x509.Certificate) {
	sha1sum := string(cert.FingerprintSHA1)
	subject := string(x509.SHA1Fingerprint(cert.RawSubject))
	issuer := string(x509.SHA1Fingerprint(cert.RawIssuer))
	pubkey := string(x509.SHA1Fingerprint(cert.RawSubjectPublicKeyInfo))

	// Certificate cached?
	if _, ok := knownCerts.Get(sha1sum); ok {
		return
	}

	var id int
	err := dbconn.QueryRow("SELECT 1 FROM raw_certificates WHERE id = $1", sha1sum).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		// not yet present
		_, err = dbconn.Exec("INSERT INTO raw_certificates (id, raw) VALUES ($1,$2)", sha1sum, cert.Raw)
		if err != nil {
			log.Panicln(err)
		}

		_, err = dbconn.Exec("INSERT INTO certificates (id, subject_id, issuer_id, key_id, first_seen_at) VALUES ($1,$2,$3,$4, NOW())", sha1sum, subject, issuer, pubkey)
		if err != nil {
			log.Panicln(err)
		}
	case err != nil:
		log.Fatal(err)
	default:
		// already present
		knownCerts.Add(sha1sum, 1)
	}

}

// Saves a HostResult in the database
func saveHostResult(result HostResult) {
	address := result.Host().String()
	certs := result.Certificates()
	starttls := result.HasStarttls()
	tlsVersion := result.TLSVersion()
	tlsCipher := result.TLSCipherSuite()

	// may also be ni
	var caFingerprints interface{}

	// Save certificates
	if certs != nil {
		// array for ca-certificate fingerprints
		fingerprints := make([][]byte, len(certs)-1)

		for i, cert := range certs {
			saveCertificate(cert)
			if i > 0 {
				// the first is the server certificate
				fingerprints[i-1] = cert.FingerprintSHA1
			}
		}

		// Cast into ByteaArray for PostgreSQL
		bytea := ByteaArray(fingerprints)
		caFingerprints = &bytea
	}

	var id int
	err := dbconn.QueryRow("SELECT id FROM mx_hosts WHERE address = $1", address).Scan(&id)

	params := []interface{}{result.ErrorString(), starttls, tlsVersion, tlsCipher, result.ServerCertificateSHA1(), caFingerprints, address}

	switch {
	case err == sql.ErrNoRows:
		// not yet present
		_, err := dbconn.Exec("INSERT INTO mx_hosts (error, starttls, tls_version, tls_cipher_suite, certificate_id, ca_certificate_ids, updated_at, address) VALUES ($1,$2,$3,$4,$5,$6, NOW(), $7)", params...)
		if err != nil {
			log.Panicln(err)
		}
	case err != nil:
		log.Fatal(err)
	default:
		_, err := dbconn.Exec("UPDATE mx_hosts SET error=$1, starttls=$2, tls_version=$3, tls_cipher_suite=$4, certificate_id=$5, ca_certificate_ids=$6, updated_at=NOW() WHERE address = $7", params...)
		if err != nil {
			log.Panicln(err)
		}
	}
}
