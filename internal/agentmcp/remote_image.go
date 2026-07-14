package agentmcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	pathpkg "path"
	"strings"
	"time"
)

const (
	maxRemoteImageBytes = 8 << 20
	remoteImageTimeout  = 15 * time.Second
	maxImageRedirects   = 3
)

var blockedRemotePrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("fec0::/10"),
}

func newRemoteImageHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialPublicAddress(ctx, dialer, network, address)
	}
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.TLSHandshakeTimeout = 5 * time.Second

	return &http.Client{
		Transport: transport,
		Timeout:   remoteImageTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxImageRedirects {
				return fmt.Errorf("image_url exceeded %d redirects", maxImageRedirects)
			}
			_, err := validateRemoteImageURL(req.URL.String())
			return err
		},
	}
}

func downloadRemoteImage(ctx context.Context, client *http.Client, rawURL string, requestedFilename string) (io.ReadCloser, string, error) {
	parsed, err := validateRemoteImageURL(rawURL)
	if err != nil {
		return nil, "", err
	}
	if client == nil {
		client = newRemoteImageHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("create image_url request: %w", err)
	}
	req.Header.Set("Accept", "image/png,image/jpeg,image/gif,image/webp")
	req.Header.Set("User-Agent", "official-account-service/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download image_url: %w", redactRemoteImageRequestError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download image_url returned status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxRemoteImageBytes {
		return nil, "", fmt.Errorf("image_url response exceeds %d bytes", maxRemoteImageBytes)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteImageBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read image_url response: %w", err)
	}
	if len(raw) > maxRemoteImageBytes {
		return nil, "", fmt.Errorf("image_url response exceeds %d bytes", maxRemoteImageBytes)
	}
	contentType := http.DetectContentType(raw)
	if !supportedRemoteImageType(contentType) {
		return nil, "", fmt.Errorf("image_url response is not a supported image: %s", contentType)
	}
	filename := remoteImageFilename(requestedFilename, parsed, resp.Header.Get("Content-Disposition"), contentType)
	return io.NopCloser(bytes.NewReader(raw)), filename, nil
}

func redactRemoteImageRequestError(err error) error {
	var requestErr *url.Error
	if errors.As(err, &requestErr) {
		return requestErr.Err
	}
	return err
}

func validateRemoteImageURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("image_url must be a public HTTPS URL")
	}
	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "" || hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return nil, fmt.Errorf("image_url must be a public HTTPS URL")
	}
	if ip := net.ParseIP(hostname); ip != nil && !isPublicRemoteIP(ip) {
		return nil, fmt.Errorf("image_url must be a public HTTPS URL")
	}
	return parsed, nil
}

func dialPublicAddress(ctx context.Context, dialer *net.Dialer, network string, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("parse image_url address: %w", err)
	}
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve image_url host: %w", err)
	}
	var lastErr error
	publicAddressFound := false
	for _, address := range addresses {
		if !isPublicRemoteIP(address) {
			continue
		}
		publicAddressFound = true
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if !publicAddressFound {
		return nil, fmt.Errorf("image_url host did not resolve to a public IP address")
	}
	return nil, fmt.Errorf("dial image_url host: %w", lastErr)
}

func isPublicRemoteIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range blockedRemotePrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func supportedRemoteImageType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func remoteImageFilename(requested string, parsed *url.URL, contentDisposition string, contentType string) string {
	if filename := cleanRemoteFilename(requested); filename != "" {
		return filename
	}
	if _, params, err := mime.ParseMediaType(contentDisposition); err == nil {
		if filename := cleanRemoteFilename(params["filename"]); filename != "" {
			return ensureImageExtension(filename, contentType)
		}
	}
	if filename := cleanRemoteFilename(pathpkg.Base(parsed.Path)); filename != "" {
		return ensureImageExtension(filename, contentType)
	}
	return "image" + imageExtension(contentType)
}

func cleanRemoteFilename(filename string) string {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	filename = pathpkg.Base(filename)
	filename = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, filename)
	if filename == "" || filename == "." || filename == "/" {
		return ""
	}
	return filename
}

func ensureImageExtension(filename string, contentType string) string {
	if pathpkg.Ext(filename) == "" {
		return filename + imageExtension(contentType)
	}
	return filename
}

func imageExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}
