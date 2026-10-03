package main

import (
	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/switcher"
)

// Automation previews have their own owner and never impersonate a Dock item.
type AutomationPreviewViewState struct {
	Open             bool                       `json:"open"`
	Session          uint64                     `json:"session"`
	Revision         uint64                     `json:"revision"`
	Title            string                     `json:"title"`
	Entries          []switcher.Entry           `json:"entries"`
	SelectedWindowID domain.WindowID            `json:"selectedWindowId"`
	Appearance       config.Appearance          `json:"appearance"`
	CardSpacingPx    int                        `json:"cardSpacingPx"`
	EmptyReason      string                     `json:"emptyReason"`
	Frames           map[domain.WindowID]string `json:"frames"`
	FrameSequence    uint64                     `json:"frameSequence"`
	Error            string                     `json:"error,omitempty"`
}
