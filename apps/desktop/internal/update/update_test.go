package update

import "testing"

func TestParseLatest(t *testing.T) {
	rel, err := ParseLatest([]byte(`{"tag_name":"v0.2.0","html_url":"https://example.com/r"}`))
	if err != nil {
		t.Fatalf("ParseLatest() error: %v", err)
	}
	if rel.Version != "v0.2.0" || rel.URL != "https://example.com/r" {
		t.Errorf("ParseLatest() = %+v", rel)
	}
}

func TestParseLatest_Assets(t *testing.T) {
	body := []byte(`{"tag_name":"v0.2.0","html_url":"https://x","assets":[` +
		`{"name":"option-tab_0.2.0_darwin_arm64.dmg","browser_download_url":"https://dl/mac"},` +
		`{"name":"option-tab_0.2.0_windows_amd64.zip","browser_download_url":"https://dl/win"}]}`)
	rel, err := ParseLatest(body)
	if err != nil {
		t.Fatalf("ParseLatest() error: %v", err)
	}
	if len(rel.Assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(rel.Assets))
	}
}

func TestRelease_AssetFor(t *testing.T) {
	rel := Release{Version: "v0.2.0", Assets: []Asset{
		{Name: "option-tab_0.2.0_darwin_arm64.dmg", DownloadURL: "https://dl/mac"},
		{Name: "option-tab_0.2.0_windows_amd64.zip", DownloadURL: "https://dl/win"},
	}}
	if got := rel.AssetFor("darwin_arm64"); got != "https://dl/mac" {
		t.Errorf("AssetFor(darwin_arm64) = %q, want the mac dmg url", got)
	}
	if got := rel.AssetFor("darwin_amd64"); got != "" {
		t.Errorf("AssetFor(darwin_amd64) = %q, want empty (no matching asset)", got)
	}
	if got := (Release{}).AssetFor("darwin_arm64"); got != "" {
		t.Errorf("AssetFor on release with no assets = %q, want empty", got)
	}
}

func TestParseLatest_Errors(t *testing.T) {
	if _, err := ParseLatest([]byte(`not json`)); err == nil {
		t.Error("malformed JSON should error")
	}
	if _, err := ParseLatest([]byte(`{"html_url":"x"}`)); err == nil {
		t.Error("missing tag_name should error")
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"0.1.0", "v0.2.0", true},
		{"v0.1.0", "0.1.1", true},
		{"0.1.0", "1.0.0", true},
		{"0.1.0", "0.1.0", false},
		{"0.2.0", "0.1.9", false},
		{"0.1.0", "v0.2.0-beta+build1", true},
		{"0.1", "0.2", true},        // short versions pad with zeros
		{"0.1.0", "garbage", false}, // malformed latest: fail safe
		{"garbage", "0.2.0", false}, // malformed current: fail safe
		{"0.1.0", "0.1.0.9", false}, // too many segments
		{"0.1.0", "0.-1.0", false},  // negative segment rejected: fail safe
		{"0.1.0", "v", false},       // empty after prefix strip: fail safe
	}
	for _, c := range cases {
		if got := Newer(c.current, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestReleaseAssetContract(t *testing.T) {
	const native = "option-tab_0.5.0_darwin_arm64.dmg"
	const universal = "option-tab_0.5.0_darwin_universal.dmg"
	cases := []struct {
		name, arch string
		assets     []Asset
		want       string
	}{
		{"universal Intel fallback", "darwin_amd64", []Asset{{Name: universal, DownloadURL: "universal"}}, "universal"},
		{"universal ARM fallback", "darwin_arm64", []Asset{{Name: universal, DownloadURL: "universal"}}, "universal"},
		{"exact preferred", "darwin_arm64", []Asset{{Name: universal, DownloadURL: "universal"}, {Name: native, DownloadURL: "native"}}, "native"},
		{"duplicate exact refuses fallback", "darwin_arm64", []Asset{{Name: native, DownloadURL: "one"}, {Name: native, DownloadURL: "two"}, {Name: universal, DownloadURL: "universal"}}, ""},
		{"duplicate universal refuses", "darwin_amd64", []Asset{{Name: universal, DownloadURL: "one"}, {Name: universal, DownloadURL: "two"}}, ""},
		{"wrong extension", "darwin_arm64", []Asset{{Name: native + ".sha256", DownloadURL: "checksum"}}, ""},
		{"wrong version", "darwin_arm64", []Asset{{Name: "option-tab_0.4.8_darwin_arm64.dmg", DownloadURL: "old"}}, ""},
		{"prefix not exact", "darwin_arm64", []Asset{{Name: "copy-" + native, DownloadURL: "copy"}}, ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := Release{Version: "v0.5.0", Assets: tt.assets}
			if got := r.AssetFor(tt.arch); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
