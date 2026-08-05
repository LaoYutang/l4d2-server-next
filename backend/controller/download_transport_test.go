package controller

import (
	"crypto/tls"
	"fmt"
	"l4d2-manager-next/logic"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestResolveDownloadDialAddress(t *testing.T) {
	tests := []struct {
		name       string
		address    string
		configured string
		want       string
	}{
		{
			name:       "Steam CDN IPv4",
			address:    "cdn.steamusercontent.com:443",
			configured: "192.0.2.10",
			want:       "192.0.2.10:443",
		},
		{
			name:       "normalized Steam CDN hostname",
			address:    "CDN.STEAMUSERCONTENT.COM.:443",
			configured: "192.0.2.10",
			want:       "192.0.2.10:443",
		},
		{
			name:       "Steam CDN IPv6",
			address:    "cdn.steamusercontent.com:8443",
			configured: "2001:db8::1",
			want:       "[2001:db8::1]:8443",
		},
		{
			name:       "no configured IP",
			address:    "cdn.steamusercontent.com:443",
			configured: "",
			want:       "cdn.steamusercontent.com:443",
		},
		{
			name:       "other domain",
			address:    "example.com:443",
			configured: "192.0.2.10",
			want:       "example.com:443",
		},
		{
			name:       "Steam CDN subdomain",
			address:    "assets.cdn.steamusercontent.com:443",
			configured: "192.0.2.10",
			want:       "assets.cdn.steamusercontent.com:443",
		},
		{
			name:       "address without port",
			address:    "cdn.steamusercontent.com",
			configured: "192.0.2.10",
			want:       "cdn.steamusercontent.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveDownloadDialAddress(tt.address, tt.configured); got != tt.want {
				t.Fatalf("resolveDownloadDialAddress(%q, %q) = %q, want %q", tt.address, tt.configured, got, tt.want)
			}
		})
	}
}

func TestDownloadHTTPTransportConnectsToConfiguredIPAndPreservesHostAndSNI(t *testing.T) {
	var gotHost string
	var gotServerName string

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.WriteHeader(http.StatusNoContent)
	}))
	server.TLS = &tls.Config{
		GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
			gotServerName = info.ServerName
			return nil, nil
		},
	}
	server.StartTLS()
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	_, port, err := net.SplitHostPort(serverURL.Host)
	if err != nil {
		t.Fatalf("split test server host: %v", err)
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := newDownloadHTTPTransport(
		func() string { return serverURL.Hostname() },
		dialer.DialContext,
	)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // Test-only certificate.
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	requestURL := fmt.Sprintf("https://%s/test", net.JoinHostPort(logic.SteamCDNHost, port))
	resp, err := client.Get(requestURL)
	if err != nil {
		t.Fatalf("request through configured Steam CDN IP: %v", err)
	}
	resp.Body.Close()

	wantHost := net.JoinHostPort(logic.SteamCDNHost, port)
	if gotHost != wantHost {
		t.Fatalf("request Host = %q, want %q", gotHost, wantHost)
	}
	if gotServerName != logic.SteamCDNHost {
		t.Fatalf("TLS SNI = %q, want %q", gotServerName, logic.SteamCDNHost)
	}
}
