package media

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseLRCMultiTagsOffsetAndStableMerge(t *testing.T) {
	data := []byte("\ufeff[ti:Original fixture]\r\n[00:01.00]One\r\n[00:03.250][00:05.00]Two\r\n[offset:-500]\r\n[00:01.00]Together")
	got, err := ParseLRC(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []Cue{{AtMS: 500, Text: "One\nTogether"}, {AtMS: 2750, Text: "Two"}, {AtMS: 4500, Text: "Two"}}
	if len(got) != len(want) {
		t.Fatalf("cues=%+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cue %d=%+v want %+v", i, got[i], want[i])
		}
	}
}

func TestParseLRCHourMinutesClampAndEnhancedLineText(t *testing.T) {
	got, err := ParseLRC([]byte("[offset:-30000]\n[00:01.0]<00:01.00>Early\n[61:02.125]Later"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != (Cue{AtMS: 0, Text: "Early"}) || got[1] != (Cue{AtMS: 3632125, Text: "Later"}) {
		t.Fatalf("unexpected cues: %+v", got)
	}
}

func TestParseLRCRejectsInvalidInput(t *testing.T) {
	cases := map[string][]byte{
		"invalid utf8":       {0xff, '[', '0', '0', ':', '0', '1', ']', 'x'},
		"no timestamps":      []byte("[ti:Fixture]\nplain text"),
		"invalid seconds":    []byte("[00:60.00]No"),
		"invalid timestamp":  []byte("[00:xx.00]No"),
		"overflow timestamp": []byte("[999999999999999999:00.00]No"),
		"overflow offset":    []byte("[offset:999999999999999999999]\n[00:01]No"),
		"adjusted overflow":  []byte("[offset:9223372036854775807]\n[00:01]No"),
		"line too long":      []byte("[00:01]" + strings.Repeat("x", 4090)),
		"file too large":     bytes.Repeat([]byte("x"), maxLRCBytes+1),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseLRC(data); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseLRCRejectsMoreThanRawCueLimit(t *testing.T) {
	var data strings.Builder
	for range maxLRCCues + 1 {
		data.WriteString("[00:01]x\n")
	}
	if _, err := ParseLRC([]byte(data.String())); err == nil {
		t.Fatal("accepted too many raw cues")
	}
}

func TestParseLRCRejectsExpandedCueTextOverLimit(t *testing.T) {
	line := strings.Repeat("[00:01]", 250) + strings.Repeat("x", 1800) + "\n"
	data := []byte(strings.Repeat(line, 3))
	if len(data) >= maxLRCBytes {
		t.Fatal("fixture must exercise expansion rather than input-size rejection")
	}
	if _, err := ParseLRC(data); err == nil {
		t.Fatal("accepted expanded cue text over 1 MiB")
	}
}

func TestActiveCueBinarySearchesBoundaries(t *testing.T) {
	cues := []Cue{{AtMS: 100, Text: "a"}, {AtMS: 200, Text: "b"}, {AtMS: 400, Text: "c"}}
	for _, tt := range []struct{ pos, want int }{{-1, -1}, {99, -1}, {100, 0}, {399, 1}, {400, 2}, {900, 2}} {
		if got := ActiveCue(cues, int64(tt.pos)); got != tt.want {
			t.Fatalf("position %d => %d want %d", tt.pos, got, tt.want)
		}
	}
}
