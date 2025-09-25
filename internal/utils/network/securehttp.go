package network

import (
	"crypto/tls"
	"net/http"
)

// NewSecureHTTPClient returns an http.Client with a custom TLS configuration.
func NewSecureHTTPClient() *http.Client {

	// Clone, to start from defaults and only override what is required
	base := http.DefaultTransport.(*http.Transport).Clone()

	// TLS policy
	base.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			// (intentionally omit non-allowed ciphers per Intel CT-35)
		},
	}

	return &http.Client{Transport: base}
}
