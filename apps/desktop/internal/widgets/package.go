package widgets

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"io"
	"slices"
	"strings"
)

const (
	MaxCompressed = 4 * 1024 * 1024
	MaxExpanded   = 8 * 1024 * 1024
	MaxAsset      = 512 * 1024
)

// Package has no grants or enabled state. Every getter returns owned copies.
type Package struct {
	digest   string
	manifest []byte
	assets   map[string][]byte
}

func (p *Package) Digest() string     { return p.digest }
func (p *Package) Manifest() Manifest { var m Manifest; _ = json.Unmarshal(p.manifest, &m); return m }

func (p *Package) Asset(id string) ([]byte, error) {
	b, ok := p.assets[id]
	if !ok {
		return nil, ErrInvalid
	}
	return slices.Clone(b), nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func boundedRead(ctx context.Context, r io.Reader, max int) ([]byte, error) {
	if ctx == nil || r == nil {
		return nil, ErrInvalid
	}
	b, err := io.ReadAll(io.LimitReader(contextReader{ctx, r}, int64(max)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > max {
		return nil, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return b, nil
}

// Preview only reads the supplied archive. It creates no files or authority.
func Preview(ctx context.Context, r io.Reader) (*Package, error) {
	raw, err := boundedRead(ctx, r, MaxCompressed)
	if err != nil {
		return nil, err
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil || len(z.File) > 64 {
		return nil, ErrInvalid
	}
	files := map[string][]byte{}
	seen := map[string]bool{}
	total := 0
	for _, f := range z.File {
		name := f.Name
		fold := strings.ToLower(name)
		if seen[fold] || !validZIPMetadata(f) || !f.Mode().IsRegular() || (name != "widget.json" && !safeAssetPath(name)) || f.Flags&1 != 0 {
			return nil, ErrInvalid
		}
		seen[fold] = true
		limit := MaxAsset
		if name == "widget.json" {
			limit = MaxManifest
		}
		if f.UncompressedSize64 > uint64(limit) || f.UncompressedSize64 > uint64(MaxExpanded-total) {
			return nil, ErrInvalid
		}
		entry, e := f.Open()
		if e != nil {
			return nil, ErrInvalid
		}
		b, e := boundedRead(ctx, entry, limit)
		closeErr := entry.Close()
		if e != nil {
			return nil, e
		}
		if closeErr != nil {
			return nil, ErrInvalid
		}
		total += len(b)
		if total > MaxExpanded {
			return nil, ErrInvalid
		}
		files[name] = b
	}
	m, err := ParseManifest(files["widget.json"])
	if err != nil {
		return nil, err
	}
	if len(files) != 1+len(m.Assets) {
		return nil, ErrInvalid
	}
	assets := map[string][]byte{}
	for _, a := range m.Assets {
		b, ok := files[a.Path]
		if !ok {
			return nil, ErrInvalid
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != a.SHA256 {
			return nil, ErrInvalid
		}
		cfg, e := png.DecodeConfig(bytes.NewReader(b))
		if e != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4_000_000/cfg.Height {
			return nil, ErrInvalid
		}
		if _, e = png.Decode(bytes.NewReader(b)); e != nil {
			return nil, ErrInvalid
		}
		assets[a.ID] = b
	}
	canonical, err := json.Marshal(m)
	if err != nil {
		return nil, ErrInvalid
	}
	h := sha256.New()
	_, _ = h.Write([]byte("OptionTabWidgetPackage1\x00"))
	writeHash := func(b []byte) {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(b)))
		_, _ = h.Write(length[:])
		_, _ = h.Write(b)
	}
	writeHash(canonical)
	ids := make([]string, 0, len(assets))
	for id := range assets {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		writeHash([]byte(id))
		writeHash(assets[id])
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return &Package{digest: hex.EncodeToString(h.Sum(nil)), manifest: canonical, assets: assets}, nil
}

// Only the timestamp extra field is useful here. Link, Unicode alternate-name,
// ZIP64 and vendor records are unnecessary within our small bounded format.
func validZIPMetadata(f *zip.File) bool {
	if f.Mode().Perm()&0o111 != 0 {
		return false
	}
	extra := f.Extra
	for len(extra) > 0 {
		if len(extra) < 4 {
			return false
		}
		id := binary.LittleEndian.Uint16(extra[:2])
		n := int(binary.LittleEndian.Uint16(extra[2:4]))
		extra = extra[4:]
		if n > len(extra) || id != 0x5455 {
			return false
		}
		extra = extra[n:]
	}
	return true
}
