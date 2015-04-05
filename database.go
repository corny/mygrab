package main

import (
	"database/sql"
	"github.com/hashicorp/golang-lru"
	_ "github.com/lib/pq"
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/x509"
	"github.com/zmap/zgrab/ztools/ztls"
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

	// Initialize cache
	if knownCerts, err = lru.New(1024); err != nil {
		panic(err)
	}

}

// Saves the certificate if it is not saved yet
func saveCertificate(cert *x509.Certificate) {
	sha1 := string([]byte(cert.FingerprintSHA1))

	// Certificate in cache?
	if _, ok := knownCerts.Get(sha1); ok {
		return
	}

	var id int
	err := dbconn.QueryRow("SELECT id FROM raw_certificates WHERE id = $1", sha1).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		// not yet present
		dbconn.QueryRow("INSERT INTO raw_certificates (id, id) VALUES ($1,$2)", id, sha1)
		// TODO insert into certificates
	case err != nil:
		log.Fatal(err)
	default:
		// already present
		knownCerts.Add(sha1, id)
	}

}

func saveOutput(grab zlib.Grab) {
	address := grab.Host.String()
	var tlsHandshake *ztls.ServerHandshake
	var lastError error

	for _, entry := range grab.Log {
		data := entry.Data
		log.Println(data.GetType())

		if entry.Error != nil {
			lastError = entry.Error
		}

		obj, ok := data.(*zlib.TLSHandshakeEvent)
		if ok {
			tlsHandshake = obj.GetHandshakeLog()
		}
	}

	log.Println("Error: ", lastError)

	if tlsHandshake != nil {
		log.Println(tlsHandshake.ServerHello.Version)
		log.Println(tlsHandshake.ServerHello.CipherSuite)

		// Save certificates
		for _, cert := range tlsHandshake.ServerCertificates.ParsedCertificates {
			saveCertificate(cert)
		}

	}

	var id int
	err := dbconn.QueryRow("SELECT id FROM mx_hosts WHERE address = $1", address).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		// not yet present
		dbconn.QueryRow("INSERT INTO mx_hosts (address) VALUES ($1)", address)
	case err != nil:
		log.Fatal(err)
	default:
	}
}
