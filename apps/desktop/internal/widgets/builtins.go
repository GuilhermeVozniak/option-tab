package widgets

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"sync"
)

var (
	builtinOnce     sync.Once
	builtinPackages []*Package
)

func builtinText(v string) *string { return &v }
func bindingNode(provider, field, formatter string) Node {
	return Node{Kind: "text", Binding: &Binding{Provider: provider, Field: field, Formatter: formatter}}
}

func initBuiltins() {
	for _, name := range []string{"clock", "battery", "network", "audio"} {
		m := Manifest{SchemaVersion: 1, ID: "org.optiontab." + name, Version: "1.0.0", MinimumAppVersion: "0.4.8", Name: Localized{"en": name}, Description: Localized{"en": "Built-in local " + name + " widget."}}
		switch name {
		case "clock":
			m.RequiredCapabilities = []string{"clock.read"}
			m.Settings = []Setting{{ID: "timezone", Type: "timezone", Name: Localized{"en": "Time zone"}, DefaultText: builtinText("Local")}, {ID: "format", Type: "choice", Name: Localized{"en": "Time format"}, DefaultText: builtinText("shortTime"), Options: []string{"shortTime", "longTime", "date"}}}
			m.Root = bindingNode("clock", "time", "shortTime")
			m.Root.Binding.TimezoneSetting = "timezone"
			m.Root.Binding.FormatterSetting = "format"
		case "battery":
			m.RequiredCapabilities = []string{"battery.read"}
			m.Root = Node{Kind: "column", Children: []Node{bindingNode("battery", "charge", "percent"), bindingNode("battery", "charging", "boolean"), bindingNode("battery", "powerSource", "text")}}
		case "network":
			m.RequiredCapabilities = []string{"network.status.read"}
			m.OptionalCapabilities = []string{"network.usage.read"}
			m.Root = Node{Kind: "column", Children: []Node{bindingNode("network", "connected", "boolean"), bindingNode("network", "category", "text"), bindingNode("network", "uploadRate", "bytesPerSecond"), bindingNode("network", "downloadRate", "bytesPerSecond")}}
		case "audio":
			m.RequiredCapabilities = []string{"audio.status.read"}
			m.OptionalCapabilities = []string{"audio.output.select"}
			m.Root = Node{Kind: "column", Children: []Node{bindingNode("audio", "outputName", "text"), bindingNode("audio", "volume", "percent"), bindingNode("audio", "muted", "boolean"), {Kind: "button", Text: "Choose output", Command: &Command{Provider: "audio", Action: "selectOutput"}}}}
		}
		raw, err := json.Marshal(m)
		if err != nil {
			panic(err)
		}
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		f, err := z.CreateHeader(&zip.FileHeader{Name: "widget.json", Method: zip.Store})
		if err != nil {
			panic(err)
		}
		if _, err = f.Write(raw); err != nil {
			panic(err)
		}
		if err = z.Close(); err != nil {
			panic(err)
		}
		p, err := Preview(context.Background(), bytes.NewReader(b.Bytes()))
		if err != nil {
			panic(err)
		}
		builtinPackages = append(builtinPackages, p)
	}
}

// Builtin returns an immutable verified package; IDs alone do not confer grants.
func Builtin(id string) (*Package, bool) {
	builtinOnce.Do(initBuiltins)
	for _, p := range builtinPackages {
		if p.Manifest().ID == id {
			return p, true
		}
	}
	return nil, false
}

func Builtins() []*Package {
	builtinOnce.Do(initBuiltins)
	return append([]*Package{}, builtinPackages...)
}
