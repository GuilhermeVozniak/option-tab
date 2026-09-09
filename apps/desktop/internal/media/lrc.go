package media

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxLRCBytes = 1 << 20
	maxLRCLine  = 4 << 10
	maxLRCCues  = 10_000
	maxLRCText  = 1 << 20
)

type Cue struct {
	AtMS int64  `json:"atMs"`
	Text string `json:"text"`
}

var (
	timestampPattern = regexp.MustCompile(`^(\d+):([0-5]\d)(?:[\.:](\d{1,3}))?$`)
	enhancedPattern  = regexp.MustCompile(`<\d+:[0-5]\d(?:[\.:]\d{1,3})?>`)
)

type rawCue struct {
	at   int64
	text string
}

func ParseLRC(data []byte) ([]Cue, error) {
	if len(data) > maxLRCBytes {
		return nil, errors.New("LRC file exceeds 1 MiB")
	}
	if !utf8.Valid(data) {
		return nil, errors.New("LRC file is not valid UTF-8")
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 1024), maxLRCLine+1)
	var raw []rawCue
	var offset int64
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if len(line) > maxLRCLine {
			return nil, errors.New("LRC line exceeds 4 KiB")
		}
		var times []int64
		rest := line
		for strings.HasPrefix(rest, "[") {
			close := strings.IndexByte(rest, ']')
			if close < 0 {
				break
			}
			tag := rest[1:close]
			rest = rest[close+1:]
			if strings.HasPrefix(strings.ToLower(tag), "offset:") {
				parsed, err := strconv.ParseInt(strings.TrimSpace(tag[len("offset:"):]), 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid LRC offset: %w", err)
				}
				offset = parsed
				continue
			}
			at, timestamp, err := parseLRCTimestamp(tag)
			if err != nil {
				return nil, err
			}
			if timestamp {
				times = append(times, at)
				continue
			}
			// Known and unknown metadata are line-level annotations, never cues.
		}
		text := enhancedPattern.ReplaceAllString(rest, "")
		for _, at := range times {
			if len(raw) >= maxLRCCues {
				return nil, errors.New("LRC file exceeds 10000 raw cues")
			}
			raw = append(raw, rawCue{at: at, text: text})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("LRC line exceeds 4 KiB")
	}
	if len(raw) == 0 {
		return nil, errors.New("LRC file has no usable timestamps")
	}
	for i := range raw {
		if (offset > 0 && raw[i].at > math.MaxInt64-offset) || (offset < 0 && raw[i].at < math.MinInt64-offset) {
			return nil, errors.New("LRC adjusted timestamp overflows")
		}
		raw[i].at += offset
		if raw[i].at < 0 {
			raw[i].at = 0
		}
	}
	sort.SliceStable(raw, func(i, j int) bool { return raw[i].at < raw[j].at })
	cues := make([]Cue, 0, len(raw))
	expanded := 0
	for start := 0; start < len(raw); {
		end := start + 1
		textBytes := len(raw[start].text)
		for end < len(raw) && raw[end].at == raw[start].at {
			if textBytes > maxLRCText-1-len(raw[end].text) {
				return nil, errors.New("LRC expanded cue text exceeds 1 MiB")
			}
			textBytes += 1 + len(raw[end].text)
			end++
		}
		if expanded > maxLRCText-textBytes {
			return nil, errors.New("LRC expanded cue text exceeds 1 MiB")
		}
		expanded += textBytes
		var text strings.Builder
		text.Grow(textBytes)
		for i := start; i < end; i++ {
			if i > start {
				text.WriteByte('\n')
			}
			text.WriteString(raw[i].text)
		}
		cues = append(cues, Cue{AtMS: raw[start].at, Text: text.String()})
		start = end
	}
	return cues, nil
}

func parseLRCTimestamp(tag string) (int64, bool, error) {
	match := timestampPattern.FindStringSubmatch(tag)
	if match == nil {
		if len(tag) > 0 && tag[0] >= '0' && tag[0] <= '9' && strings.Contains(tag, ":") {
			return 0, false, fmt.Errorf("invalid LRC timestamp %q", tag)
		}
		return 0, false, nil
	}
	minutes, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil || minutes > math.MaxInt64/60_000 {
		return 0, false, fmt.Errorf("invalid LRC timestamp %q", tag)
	}
	seconds, _ := strconv.ParseInt(match[2], 10, 64)
	fraction := match[3]
	for len(fraction) < 3 {
		fraction += "0"
	}
	millis := int64(0)
	if fraction != "" {
		millis, _ = strconv.ParseInt(fraction, 10, 64)
	}
	base := minutes * 60_000
	addition := seconds*1000 + millis
	if base > math.MaxInt64-addition {
		return 0, false, fmt.Errorf("invalid LRC timestamp %q", tag)
	}
	return base + addition, true, nil
}

func ActiveCue(cues []Cue, positionMS int64) int {
	index := sort.Search(len(cues), func(i int) bool { return cues[i].AtMS > positionMS }) - 1
	return index
}
