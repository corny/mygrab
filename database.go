package main

import (
	"database/sql"
	"github.com/hashicorp/golang-lru"
	_ "github.com/lib/pq"
	"github.com/zmap/zgrab/ztools/x509"
	"log"
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
	tlsHandshake := result.TLSHandshake
	starttls := result.HasStarttls()
	tlsVersion := result.TLSVersion()
	tlsCipher := result.TLSCipherSuite()

	log.Println(address)

	// Save certificates
	if certs != nil {
		for _, cert := range tlsHandshake.ServerCertificates.ParsedCertificates {
			saveCertificate(cert)
		}
		// log.Println(certs[0].PublicKeyAlgorithm)
	}

	var id int
	err := dbconn.QueryRow("SELECT id FROM mx_hosts WHERE address = $1", address).Scan(&id)

	params := []interface{}{result.ErrorString(), starttls, tlsVersion, tlsCipher, result.ServerCertificateSHA1(), address}

	switch {
	case err == sql.ErrNoRows:
		// not yet present
		_, err := dbconn.Exec("INSERT INTO mx_hosts (error, starttls, tls_version, tls_cipher_suite, certificate_id, updated_at, address) VALUES ($1,$2,$3,$4,$5, NOW(), $6)", params...)
		if err != nil {
			log.Panicln(err)
		}
	case err != nil:
		log.Fatal(err)
	default:
		_, err := dbconn.Exec("UPDATE mx_hosts SET error=$1, starttls=$2, tls_version=$3, tls_cipher_suite=$4, certificate_id=$5, updated_at=NOW() WHERE address = $6", params...)
		if err != nil {
			log.Panicln(err)
		}
	}
}
