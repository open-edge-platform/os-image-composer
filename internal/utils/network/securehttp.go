package network

import (
	"crypto/tls"
	"net/http"
)

// NewSecureHTTPClient returns an http.Client with a custom TLS configuration.
// Callers can reuse this instead of re-defining the TLS settings everywhere.
func NewSecureHTTPClient() *http.Client {

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,

		// CipherSuites applies only to TLS 1.0–1.2
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			// (intentionally omit non-allowed ciphers per Intel CT-35)
		},
	}

	transport := &http.Transport{
		TLSClientConfig:   tlsConfig,
		ForceAttemptHTTP2: true,
	}

	return &http.Client{
		Transport: transport,
	}
}
