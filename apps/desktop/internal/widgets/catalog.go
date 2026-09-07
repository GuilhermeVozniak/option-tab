package widgets

import "slices"

type fieldSpec struct{ capability, kind string }

var fields = map[string]fieldSpec{
	"clock.time":     {"clock.read", "time"},
	"battery.charge": {"battery.read", "fraction"}, "battery.charging": {"battery.read", "boolean"}, "battery.powerSource": {"battery.read", "text"},
	"network.connected": {"network.status.read", "boolean"}, "network.category": {"network.status.read", "text"}, "network.uploadRate": {"network.usage.read", "rate"}, "network.downloadRate": {"network.usage.read", "rate"},
	"audio.outputName": {"audio.status.read", "text"}, "audio.volume": {"audio.status.read", "fraction"}, "audio.muted": {"audio.status.read", "boolean"},
	"music.title": {"media.music.read", "text"}, "music.artist": {"media.music.read", "text"}, "music.album": {"media.music.read", "text"}, "music.position": {"media.music.read", "duration"}, "music.duration": {"media.music.read", "duration"}, "music.playback": {"media.music.read", "text"},
	"spotify.title": {"media.spotify.read", "text"}, "spotify.artist": {"media.spotify.read", "text"}, "spotify.album": {"media.spotify.read", "text"}, "spotify.position": {"media.spotify.read", "duration"}, "spotify.duration": {"media.spotify.read", "duration"}, "spotify.playback": {"media.spotify.read", "text"},
}

func validBinding(b Binding, caps map[string]bool, settings map[string]string, numeric bool) bool {
	f, ok := fields[b.Provider+"."+b.Field]
	if !ok || !caps[f.capability] {
		return false
	}
	if b.TimezoneSetting != "" && (f.kind != "time" || settings[b.TimezoneSetting] != "timezone") {
		return false
	}
	if numeric && !slices.Contains([]string{"fraction", "rate", "duration"}, f.kind) {
		return false
	}
	switch f.kind {
	case "time":
		return slices.Contains([]string{"shortTime", "longTime", "date"}, b.Formatter)
	case "fraction":
		return b.Formatter == "percent" || b.Formatter == "number"
	case "rate":
		return b.Formatter == "bytesPerSecond" || b.Formatter == "number"
	case "duration":
		return b.Formatter == "duration" || b.Formatter == "number"
	case "boolean":
		return b.Formatter == "boolean"
	case "text":
		return b.Formatter == "text"
	}
	return false
}

func validCommand(c Command, caps map[string]bool) bool {
	if c.Provider == "audio" {
		return c.Action == "selectOutput" && caps["audio.output.select"]
	}
	if c.Provider == "music" || c.Provider == "spotify" {
		return caps["media."+c.Provider+".control"] && slices.Contains([]string{"play", "pause", "playPause", "next", "previous", "seek"}, c.Action)
	}
	return false
}
