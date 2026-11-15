//go:build js

package library

import "net/http"

// linkTransport is the browser's fetch on the web build: the browser connects, and webURL has refused the addresses
// written as this machine's or a private network's.
func linkTransport() http.RoundTripper { return http.DefaultTransport }
