package network

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewSecureHTTPClient returns an http.Client with a custom TLS configuration.
func NewSecureHTTPClient() *http.Client {

	// Clone, to start from defaults and only override what is required
	base := http.DefaultTransport.(*http.Transport).Clone()

	// Robust dialing timeouts
	base.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second, // TCP connect timeout
		KeepAlive: 30 * time.Second,
	}).DialContext

	// Configiure other standard timeouts
	base.TLSHandshakeTimeout = 10 * time.Second
	base.ResponseHeaderTimeout = 15 * time.Second
	base.ExpectContinueTimeout = 1 * time.Second
	base.IdleConnTimeout = 90 * time.Second
	base.ForceAttemptHTTP2 = true

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

	return &http.Client{
		Transport: base,
		Timeout:   30 * time.Second, // end-to-end request deadline
	}
}
