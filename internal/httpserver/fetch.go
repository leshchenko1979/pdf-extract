package httpserver

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func blockedHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return true
	}
	if h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "0.0.0.0" {
		return true
	}
	if strings.HasSuffix(h, ".localhost") {
		return true
	}
	if strings.HasSuffix(h, ".local") {
		return true
	}
	return false
}

// DownloadPDF fetches a PDF from urlStr, saves to destPath, enforces max bytes.
func DownloadPDF(client *http.Client, urlStr string, maxBytes int64, destPath string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https URLs are allowed")
	}
	if blockedHost(u.Hostname()) {
		return fmt.Errorf("url host is not allowed")
	}
	ips, err := net.LookupIP(u.Hostname())
	if err == nil {
		for _, ip := range ips {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				return fmt.Errorf("url resolves to a disallowed address")
			}
		}
	}

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	lim := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(lim)
	if err != nil {
		return err
	}
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("pdf exceeds max download size")
	}
	if len(data) < 5 {
		return fmt.Errorf("response too small to be a PDF")
	}
	if string(data[:5]) != "%PDF-" {
		return fmt.Errorf("response is not a PDF")
	}
	return os.WriteFile(destPath, data, 0o600)
}

// NewFetchClient returns an HTTP client for outbound PDF fetches.
func NewFetchClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL != nil && blockedHost(req.URL.Hostname()) {
				return fmt.Errorf("redirect to disallowed host")
			}
			return nil
		},
	}
}
