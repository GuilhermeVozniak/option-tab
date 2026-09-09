package launcher

import (
	"errors"
	"time"
	"unicode/utf8"

	"option-tab/internal/config"
)

// ValidateWidgetNode accepts trusted renderer primitives only, never code or URLs.
func ValidateWidgetNode(root WidgetNode) error {
	count := 0
	var visit func(WidgetNode, int) bool
	visit = func(n WidgetNode, depth int) bool {
		count++
		if count > 16 || depth > 4 || !utf8.ValidString(n.Text) || utf8.RuneCountInString(n.Text) > 80 {
			return false
		}
		switch n.Kind {
		case "row":
			if n.Text != "" || len(n.Children) > 16 {
				return false
			}
			for _, c := range n.Children {
				if !visit(c, depth+1) {
					return false
				}
			}
			return true
		case "text":
			return len(n.Children) == 0
		default:
			return false
		}
	}
	if !visit(root, 1) {
		return errors.New("launcher: invalid widget node")
	}
	return nil
}

func widgets(p config.LauncherProfile, visible bool, now time.Time) []Widget {
	out := []Widget{}
	for _, w := range p.Widgets {
		status := "disabled"
		root := WidgetNode{Kind: "row"}
		if w.Enabled && !granted(w) {
			status = "grantRequired"
		}
		if granted(w) && visible {
			status = "ready"
			root.Children = []WidgetNode{{Kind: "text", Text: now.Format("15:04")}}
		}
		out = append(out, Widget{ID: w.ID, PackageID: w.PackageID, Digest: w.Digest, Status: status, Root: root})
	}
	return out
}
