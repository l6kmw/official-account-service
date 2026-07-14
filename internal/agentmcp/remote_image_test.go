package agentmcp

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenUploadContentDownloadsRemoteImage(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "https://cdn.example.com/uploads/cover.png?signature=temporary", r.URL.String())
		return remoteImageResponse(http.StatusOK, "image/png", pngImageBytes()), nil
	})}

	content, filename, err := openUploadContent(context.Background(), uploadImageToolInput{
		Usage:    "cover",
		ImageURL: "https://cdn.example.com/uploads/cover.png?signature=temporary",
	}, "", client)
	require.NoError(t, err)
	defer content.Close()

	raw, err := io.ReadAll(content)
	require.NoError(t, err)
	require.Equal(t, pngImageBytes(), raw)
	require.Equal(t, "cover.png", filename)
}

func TestOpenUploadContentRequiresExactlyOneSource(t *testing.T) {
	for _, input := range []uploadImageToolInput{
		{Usage: "cover"},
		{Usage: "cover", FilePath: "/tmp/image.png", ImageURL: "https://cdn.example.com/image.png"},
		{Usage: "cover", ContentBase64: "aW1hZ2U=", ImageURL: "https://cdn.example.com/image.png"},
	} {
		_, _, err := openUploadContent(context.Background(), input, "", nil)
		require.ErrorContains(t, err, "provide exactly one of file_path, content_base64, or image_url")
	}
}

func TestOpenUploadContentRejectsInvalidRemoteImage(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "not an image", body: "plain text", want: "response is not a supported image"},
		{name: "too large", body: strings.Repeat("x", maxRemoteImageBytes+1), want: "exceeds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				response := remoteImageResponse(http.StatusOK, "application/octet-stream", []byte(tt.body))
				response.ContentLength = -1
				return response, nil
			})}
			_, _, err := openUploadContent(context.Background(), uploadImageToolInput{
				Usage:    "inline_image",
				ImageURL: "https://cdn.example.com/image",
			}, "", client)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestRemoteImageDownloadErrorRedactsSignedURL(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network failed")
	})}

	_, _, err := openUploadContent(context.Background(), uploadImageToolInput{
		Usage:    "cover",
		ImageURL: "https://cdn.example.com/image.png?signature=do-not-log",
	}, "", client)
	require.ErrorContains(t, err, "network failed")
	require.NotContains(t, err.Error(), "do-not-log")
}

func TestValidateRemoteImageURLRejectsUnsafeTargets(t *testing.T) {
	for _, rawURL := range []string{
		"http://cdn.example.com/image.png",
		"https://localhost/image.png",
		"https://127.0.0.1/image.png",
		"https://169.254.169.254/latest/meta-data",
		"https://[::1]/image.png",
		"file:///tmp/image.png",
	} {
		t.Run(rawURL, func(t *testing.T) {
			_, err := validateRemoteImageURL(rawURL)
			require.Error(t, err)
		})
	}

	parsed, err := validateRemoteImageURL("https://cdn.example.com/image.png")
	require.NoError(t, err)
	require.Equal(t, "cdn.example.com", parsed.Hostname())
}

func TestRemoteImageClientRejectsUnsafeRedirect(t *testing.T) {
	client := newRemoteImageHTTPClient()
	redirect, err := http.NewRequest(http.MethodGet, "https://127.0.0.1/private.png", nil)
	require.NoError(t, err)
	original, err := http.NewRequest(http.MethodGet, "https://cdn.example.com/image.png", nil)
	require.NoError(t, err)

	err = client.CheckRedirect(redirect, []*http.Request{original})
	require.ErrorContains(t, err, "public HTTPS")
}

func TestPublicRemoteIPValidation(t *testing.T) {
	for _, rawIP := range []string{"0.1.2.3", "127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.169.254", "192.168.1.2", "::1", "64:ff9b::a00:1", "fc00::1", "fe80::1", "fec0::1"} {
		require.False(t, isPublicRemoteIP(net.ParseIP(rawIP)), rawIP)
	}
	require.True(t, isPublicRemoteIP(net.ParseIP("8.8.8.8")))
	require.True(t, isPublicRemoteIP(net.ParseIP("2606:4700:4700::1111")))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func remoteImageResponse(status int, contentType string, body []byte) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Header:        http.Header{"Content-Type": []string{contentType}},
		Body:          io.NopCloser(strings.NewReader(string(body))),
		ContentLength: int64(len(body)),
	}
}

func pngImageBytes() []byte {
	return []byte("\x89PNG\r\n\x1a\n")
}
