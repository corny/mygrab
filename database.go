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

// Saves the certificate if it is not saved yet
func saveCertificate(cert *x509.Certificate) {
	sha1 := string(cert.FingerprintSHA1)

	// Certificate cached?
	if _, ok := knownCerts.Get(sha1); ok {
		return
	}

	var id int
	err := dbconn.QueryRow("SELECT 1 FROM raw_certificates WHERE id = $1", sha1).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		// not yet present
		_, err = dbconn.Exec("INSERT INTO raw_certificates (id, raw) VALUES ($1,$2)", sha1, cert.Raw)
		if err != nil {
			log.Panicln(err)
		}
		// TODO insert into certificates
	case err != nil:
		log.Fatal(err)
	default:
		// already present
		knownCerts.Add(sha1, 1)
	}

}

func saveOutput(result HostResult) {
	address := result.Host().String()
	certs := result.Certificates()

	log.Println(address)

	// Save certificates
	if certs != nil {
		for _, cert := range tlsHandshake.ServerCertificates.ParsedCertificates {
			saveCertificate(cert)
		}
	}

	var id int
	err := dbconn.QueryRow("SELECT id FROM mx_hosts WHERE address = $1", address).Scan(&id)

	switch {
	case err == sql.ErrNoRows:
		// not yet present
		_, err := dbconn.Exec("INSERT INTO mx_hosts (error, starttls, tls_version, tls_cipher_suite, certificate_id, address) VALUES ($1,$2,$3,$4,$5,$6)", result.ErrorString(), result.HasStarttls(), result.TLSVersion(), result.TLSCipherSuite(), result.ServerCertificateSHA1(), address)
		if err != nil {
			log.Panicln(err)
		}
	case err != nil:
		log.Fatal(err)
	default:
		_, err := dbconn.Exec("UPDATE mx_hosts SET error=$1, starttls=$2, tls_version=$3, tls_cipher_suite=$4, certificate_id=$5 WHERE address = $6", result.ErrorString(), result.HasStarttls(), result.TLSVersion(), result.TLSCipherSuite(), result.ServerCertificateSHA1(), address)
		if err != nil {
			log.Panicln(err)
		}
	}
}
