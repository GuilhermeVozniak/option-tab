package main

import (
	"option-tab/internal/config"
)

type LauncherInteractionCapabilities struct {
	GestureAvailable     bool   `json:"gestureAvailable"`
	PinchAvailable       bool   `json:"pinchAvailable"`
	SwipeAvailable       bool   `json:"swipeAvailable"`
	LetterInputAvailable bool   `json:"letterInputAvailable"`
	HapticsAvailable     bool   `json:"hapticsAvailable"`
	Reason               string `json:"reason"`
}
type LauncherInteractionState struct {
	Configured           config.LauncherInteractions `json:"configured"`
	Epoch                uint64                      `json:"epoch"`
	DisplayUUID          string                      `json:"displayUUID"`
	Session              uint64                      `json:"session"`
	PresentationRevision uint64                      `json:"presentationRevision"`
	Admission            uint64                      `json:"admission"`
	Sequence             uint64                      `json:"sequence"`
	Visible              bool                        `json:"visible"`
	SelectedItemID       string                      `json:"selectedItemID"`
	KeyboardMode         bool                        `json:"keyboardMode"`
	LauncherInteractionCapabilities
}
