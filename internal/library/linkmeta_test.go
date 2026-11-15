package library

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFetchLinkMeta: a page's Open Graph title and picture and its icon are read from its head, made absolute; a page
// with no icon has its site's /favicon.ico; what is not a web address is not asked for (#495).
func TestFetchLinkMeta(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><html><head>
<title>  Plain   title </title>
<meta property="og:title" content="Dune &amp; Arrakis">
<meta name="twitter:image" content="/tw.png">
<meta property="og:image" content="pics/og.png">
<link rel="mask-icon" href="/mask.svg">
<link rel="icon" type="image/svg+xml" href="/icon.svg">
<link rel="apple-touch-icon" href="/apple.png">
<link rel="shortcut icon" href="/fav.png?v=2">
</head><body><meta property="og:image" content="/late.png"></body></html>`))
	})
	mux.HandleFunc("/bare", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Bare</title></head><body>x</body></html>`))
	})
	mux.HandleFunc("/moved", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/bare", http.StatusFound) })
	mux.HandleFunc("/pic.png", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("PNG")) })
	mux.HandleFunc("/huge.png", func(w http.ResponseWriter, r *http.Request) { w.Write(make([]byte, linkImageMax+10)) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	localLinks(t)
	ctx := context.Background()

	m, err := FetchLinkMeta(ctx, srv.URL+"/blog/../post")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Dune & Arrakis" || m.Image != srv.URL+"/pics/og.png" || m.Icon != srv.URL+"/fav.png?v=2" {
		t.Fatalf("read: %+v", m)
	}
	m, err = FetchLinkMeta(ctx, srv.URL+"/moved")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Bare" || m.Image != "" || m.Icon != srv.URL+"/favicon.ico" {
		t.Fatalf("a page with nothing but a title, after a redirect: %+v", m)
	}
	if _, err := FetchLinkMeta(ctx, srv.URL+"/missing"); err == nil {
		t.Fatal("a page that is not there has its meta")
	}
	for _, raw := range []string{"mailto:a@b.c", "page://notlar/dune", "file:///etc/passwd", "https://"} {
		if _, err := FetchLinkMeta(ctx, raw); err == nil {
			t.Fatalf("%s was asked for", raw)
		}
	}
	if data, err := FetchLinkImage(ctx, srv.URL+"/pic.png"); err != nil || string(data) != "PNG" {
		t.Fatalf("the picture: %q %v", data, err)
	}
	if _, err := FetchLinkImage(ctx, srv.URL+"/huge.png"); err == nil || !strings.Contains(err.Error(), "large") {
		t.Fatalf("a picture too large was taken: %v", err)
	}
}

// localLinks lets links be fetched from a test's own server on 127.0.0.1 while the test runs.
func localLinks(t *testing.T) {
	AllowLocalLinks = true
	t.Cleanup(func() { AllowLocalLinks = false })
}
