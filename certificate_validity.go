package main

import (
	"errors"
	"github.com/deckarep/golang-set"
	"github.com/zmap/zgrab/ztools/x509"
	"time"
)

type CertificateValidity struct {
	Expired       bool // Expiration of the server certificate
	Error         error
	Certificates  []*x509.Certificate            // the first is the server certificate
	TrustedChains map[string][]*x509.Certificate // map from trusted root store to chain
}

func NewCertificateValidity(certs []*x509.Certificate) *CertificateValidity {
	v := &CertificateValidity{
		Certificates:  certs,
		TrustedChains: make(map[string][]*x509.Certificate),
	}

	var leaf *x509.Certificate

	opts := x509.VerifyOptions{
		CurrentTime:   time.Now(),
		Intermediates: x509.NewCertPool(),
	}

	for i, cert := range certs {
		if i == 0 {
			leaf = cert
		} else {
			opts.Intermediates.AddCert(cert)
		}
	}

	// Check expiration
	v.Expired = opts.CurrentTime.Before(leaf.NotBefore) || opts.CurrentTime.After(leaf.NotAfter)

	// Check for unhandled critical extensions
	if len(leaf.UnhandledCriticalExtensions) > 0 {
		v.Error = errors.New("unhandled critical extension")
		return v
	}

	// Build chains to root certificates
	candidateChains, err := leaf.BuildChains(&opts)
	if err != nil {
		v.Error = err
		return v
	}

	// Filter chains by key usage
	keyUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	chains := x509.FilterChainsByKeyUsage(candidateChains, keyUsages)

	// Any chains left?
	if len(chains) == 0 {
		v.Error = x509.CertificateInvalidError{leaf, x509.IncompatibleUsage}
	}

	// Set the first chain
	// 'system' is the name of the root store
	v.TrustedChains["system"] = chains[0]

	return v
}

// Names of trusted root stores
func (v *CertificateValidity) TrustedNames() mapset.Set {
	set := mapset.NewThreadUnsafeSet()
	for key, _ := range v.TrustedChains {
		set.Add(key)
	}
	return set
}
