package platform

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func mediaArtworkPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: 150, G: 40, B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestMediaArtworkNormalizesBoundedPNGWithoutUpscaling(t *testing.T) {
	for _, tc := range []struct{ w, h, wantW, wantH int }{{1024, 512, 512, 256}, {100, 50, 100, 50}} {
		data, err := normalizeMediaArtworkPNG(mediaArtworkPNG(t, tc.w, tc.h))
		if err != nil {
			t.Fatal(err)
		}
		got, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if got.Bounds().Dx() != tc.wantW || got.Bounds().Dy() != tc.wantH {
			t.Fatalf("incorrect artwork dimensions: %v", got.Bounds())
		}
		if color.NRGBAModel.Convert(got.At(0, 0)) != (color.NRGBA{R: 150, G: 40, B: 80, A: 255}) {
			t.Fatal("solid artwork color changed")
		}
	}
	for _, data := range [][]byte{nil, []byte("not an image"), make([]byte, 5*1024*1024+1), mediaArtworkPNG(t, 4097, 1)} {
		if _, err := normalizeMediaArtworkPNG(data); err == nil {
			t.Fatal("invalid or oversized artwork decoded")
		}
	}
}

func TestMediaArtworkDialRejectsPrivateAndMixedDNSBeforeConnecting(t *testing.T) {
	for _, addresses := range [][]string{{"127.0.0.1"}, {"10.1.2.3"}, {"169.254.169.254"}, {"100.64.0.1"}, {"192.0.2.1"}, {"198.18.0.1"}, {"224.0.0.1"}, {"::1"}, {"fc00::1"}, {"fe80::1"}, {"::ffff:127.0.0.1"}, {"2001:db8::1"}, {"2002:7f00:1::"}, {"8.8.8.8", "10.0.0.1"}} {
		t.Run(strings.Join(addresses, ","), func(t *testing.T) {
			calls := 0
			client := newMediaArtworkClient(func(context.Context, string) ([]netip.Addr, error) {
				ips := make([]netip.Addr, len(addresses))
				for i, addr := range addresses {
					ips[i] = netip.MustParseAddr(addr)
				}
				return ips, nil
			}, func(context.Context, string, string) (net.Conn, error) {
				calls++
				return nil, errors.New("unexpected dial")
			})
			_, err := client.Transport.(*http.Transport).DialContext(context.Background(), "tcp", "art.example:443")
			if err == nil || calls != 0 {
				t.Fatalf("non-public artwork address reached socket: %v calls=%d", err, calls)
			}
		})
	}
	var dialed string
	client := newMediaArtworkClient(func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}, func(_ context.Context, _, address string) (net.Conn, error) {
		dialed = address
		return nil, errors.New("fixture dial")
	})
	_, _ = client.Transport.(*http.Transport).DialContext(context.Background(), "tcp", "art.example:443")
	if dialed != "8.8.8.8:443" {
		t.Fatalf("validated address was re-resolved or changed: %q", dialed)
	}
}

type mediaArtworkRoundTrip func(*http.Request) (*http.Response, error)

func (f mediaArtworkRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMediaArtworkFetchRefusesUnsafeURLsRedirectsAndOversize(t *testing.T) {
	for _, raw := range []string{"http://art.example/a", "file:///tmp/art.png", "https://user:secret@art.example/a", "https://127.0.0.1/a", "https://art.example:8443/a", "https://[::1]/a"} {
		calls := 0
		client := newMediaArtworkClient(nil, nil)
		client.Transport = mediaArtworkRoundTrip(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected request") })
		if _, err := fetchMediaArtwork(context.Background(), client, raw); err == nil || calls != 0 {
			t.Fatalf("unsafe URL reached HTTP transport: %q err=%v calls=%d", raw, err, calls)
		}
	}
	for _, location := range []string{"http://art.example/next", "https://127.0.0.1/private", "https://user:secret@art.example/next"} {
		calls := 0
		client := newMediaArtworkClient(nil, nil)
		client.Transport = mediaArtworkRoundTrip(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {location}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		})
		if _, err := fetchMediaArtwork(context.Background(), client, "https://art.example/cover"); err == nil || calls != 1 {
			t.Fatalf("unsafe redirect followed: %s err=%v calls=%d", location, err, calls)
		}
	}
	client := newMediaArtworkClient(nil, nil)
	client.Transport = mediaArtworkRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: -1, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 5*1024*1024+1))), Request: r}, nil
	})
	if _, err := fetchMediaArtwork(context.Background(), client, "https://art.example/cover"); err == nil {
		t.Fatal("unbounded artwork body accepted")
	}
}

func TestMediaArtworkFetchReturnsDecodedPNGAndHonorsCancellation(t *testing.T) {
	data := mediaArtworkPNG(t, 2, 1)
	client := newMediaArtworkClient(nil, nil)
	client.Transport = mediaArtworkRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: int64(len(data)), Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
	})
	got, err := fetchMediaArtwork(context.Background(), client, "https://art.example/cover")
	if err != nil || len(got) == 0 {
		t.Fatalf("valid artwork rejected: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fetchMediaArtwork(ctx, client, "https://art.example/cover"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled artwork fetch: %v", err)
	}
}

type mediaArtworkReaderFunc func([]byte) (int, error)

var artworkDecoderFixtureID atomic.Uint64

func (f mediaArtworkReaderFunc) Read(p []byte) (int, error) { return f(p) }

func TestMediaArtworkFetchPreservesCancellationDuringBodyRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := newMediaArtworkClient(nil, nil)
	client.Transport = mediaArtworkRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := mediaArtworkReaderFunc(func([]byte) (int, error) { cancel(); return 0, io.ErrUnexpectedEOF })
		return &http.Response{StatusCode: 200, Body: io.NopCloser(body), Request: r}, nil
	})
	if _, err := fetchMediaArtwork(ctx, client, "https://art.example/cover"); !errors.Is(err, context.Canceled) {
		t.Fatalf("body-read cancellation lost: %v", err)
	}
}

func TestMediaArtworkFetchDiscardsCancellationDuringDecode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// A test-only decoder cancels inside the otherwise noninterruptible image
	// decode boundary. No timers or production decoder substitutions are needed.
	magic := "OPTION-TAB-ART-CANCEL-" + strconv.FormatUint(artworkDecoderFixtureID.Add(1), 10) + ":"
	image.RegisterFormat(magic, magic, func(io.Reader) (image.Image, error) {
		cancel()
		return image.NewNRGBA(image.Rect(0, 0, 2, 1)), nil
	}, func(io.Reader) (image.Config, error) {
		return image.Config{ColorModel: color.NRGBAModel, Width: 2, Height: 1}, nil
	})
	client := newMediaArtworkClient(nil, nil)
	client.Transport = mediaArtworkRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(magic)), Request: r}, nil
	})
	if png, err := fetchMediaArtwork(ctx, client, "https://art.example/cover"); !errors.Is(err, context.Canceled) || len(png) != 0 {
		t.Fatalf("cancelled decoded artwork escaped: %v bytes=%d", err, len(png))
	}
}
