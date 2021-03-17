package https

import (
	"net"
	"net/http"
	"time"
)

// NewHTTPClient creates a new http client with reasonable and safe defaults
func NewHTTPClient() *http.Client {
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	client := &http.Client{
		Timeout:   time.Second * 5,
		Transport: transport,
	}
	return client
}
