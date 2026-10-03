package platform

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/image/draw"
)

const maxMediaArtworkBytes = 5 * 1024 * 1024

var mediaArtworkReserved = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
}

func publicMediaArtworkIP(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.Zone() != "" {
		return false
	}
	if address.Is6() && !netip.MustParsePrefix("2000::/3").Contains(address) {
		return false
	}
	for _, reserved := range mediaArtworkReserved {
		if reserved.Contains(address) {
			return false
		}
	}
	return true
}

func validMediaArtworkURL(value *url.URL) bool {
	if value == nil || value.Scheme != "https" || value.User != nil || value.Hostname() == "" || (value.Port() != "" && value.Port() != "443") || value.Opaque != "" {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(value.Hostname(), "."))
	if address, err := netip.ParseAddr(host); err == nil {
		return publicMediaArtworkIP(address)
	}
	return strings.Contains(host, ".") && !strings.HasSuffix(host, ".localhost") && !strings.HasSuffix(host, ".local") && !strings.HasSuffix(host, ".home.arpa")
}

// The dial boundary pins a validated address, so a second DNS lookup cannot
// substitute a private endpoint. Environment proxies are deliberately unused.
func newMediaArtworkClient(lookup func(context.Context, string) ([]netip.Addr, error), dial func(context.Context, string, string) (net.Conn, error)) *http.Client {
	if lookup == nil {
		lookup = func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		}
	}
	if dial == nil {
		dial = (&net.Dialer{Timeout: 5 * time.Second}).DialContext
	}
	transport := &http.Transport{
		DisableKeepAlives: true, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 64 * 1024,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil || port != "443" {
				return nil, errors.New("artwork destination is unavailable")
			}
			var addresses []netip.Addr
			if ip, e := netip.ParseAddr(host); e == nil {
				addresses = []netip.Addr{ip}
			} else {
				addresses, err = lookup(ctx, host)
				if err != nil {
					return nil, errors.New("artwork host could not be resolved")
				}
			}
			if len(addresses) == 0 {
				return nil, errors.New("artwork host has no public address")
			}
			for _, ip := range addresses {
				if !publicMediaArtworkIP(ip) {
					return nil, errors.New("artwork destination must be public")
				}
			}
			for _, ip := range addresses {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				conn, e := dial(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
				if e == nil {
					return conn, nil
				}
			}
			return nil, errors.New("artwork host could not be reached")
		},
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || !validMediaArtworkURL(req.URL) {
			return errors.New("artwork redirect is unavailable")
		}
		return nil
	}}
}

func fetchMediaArtwork(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("artwork context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	address, err := url.Parse(rawURL)
	if err != nil || !validMediaArtworkURL(address) {
		return nil, errors.New("artwork requires a public HTTPS image URL")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address.String(), nil)
	if err != nil {
		return nil, errors.New("artwork request is unavailable")
	}
	request.Header.Set("Accept", "image/png, image/jpeg, image/gif")
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("artwork download failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("artwork server did not return an image")
	}
	if response.ContentLength > maxMediaArtworkBytes {
		return nil, errors.New("artwork exceeds the download limit")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxMediaArtworkBytes+1))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, errors.New("artwork download was interrupted")
	}
	normalized, err := normalizeMediaArtworkPNG(data)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return normalized, err
}

// Decode dimensions before pixels, then reduce to a static PNG. Individual
// decoder calls are not interruptible, but both encoded and pixel sizes bound
// their input. No decoded bitmap or original response is retained in a cache.
func normalizeMediaArtworkPNG(data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > maxMediaArtworkBytes {
		return nil, errors.New("artwork exceeds the image limit")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 {
		return nil, errors.New("artwork image dimensions are unavailable or too large")
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("artwork could not be decoded")
	}
	w, h := source.Bounds().Dx(), source.Bounds().Dy()
	if w != config.Width || h != config.Height {
		return nil, errors.New("artwork image dimensions changed")
	}
	if max(w, h) > 512 {
		if w >= h {
			h = max(1, h*512/w)
			w = 512
		} else {
			w = max(1, w*512/h)
			h = 512
		}
	}
	target := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(target, target.Bounds(), source, source.Bounds(), draw.Src, nil)
	var result bytes.Buffer
	if err := png.Encode(&result, target); err != nil {
		return nil, errors.New("artwork could not be encoded")
	}
	return result.Bytes(), nil
}
