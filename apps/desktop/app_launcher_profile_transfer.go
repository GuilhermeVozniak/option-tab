package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"option-tab/internal/config"
)

type LauncherProfileImportReview struct {
	Digest      string   `json:"digest"`
	Revision    string   `json:"revision"`
	Name        string   `json:"name"`
	ItemCount   int      `json:"itemCount"`
	WidgetCount int      `json:"widgetCount"`
	Notices     []string `json:"notices"`
}
type LauncherProfileImportResult struct {
	ProfileID    string `json:"profileID"`
	SettingsJSON string `json:"settingsJSON"`
}

func launcherTransferError(code string) error { return errors.New("launcher profile: " + code) }
func launcherTransferDigest(b []byte) string  { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func launcherTransferRevision(s config.ReplacementDockSettings) string {
	b, _ := json.Marshal(s)
	return launcherTransferDigest(b)
}

func (a *App) GetLauncherProfileExport(profileID string) (string, error) {
	for _, p := range a.settingsSnapshot().ReplacementDock.Profiles {
		if p.ID == profileID {
			b, err := config.ExportLauncherProfile(p)
			if err != nil {
				return "", launcherTransferError("invalidDocument")
			}
			return string(b), nil
		}
	}
	return "", launcherTransferError("unavailable")
}

func (a *App) PreviewLauncherProfileImport(document string) (LauncherProfileImportReview, error) {
	p, err := config.ParseLauncherProfile([]byte(document))
	if err != nil {
		return LauncherProfileImportReview{}, launcherTransferError("invalidDocument")
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if !a.launcherChoicesAllowedLocked() {
		return LauncherProfileImportReview{}, launcherTransferError("unavailable")
	}
	s := a.settingsSnapshot().ReplacementDock
	if len(s.Profiles) >= 8 {
		return LauncherProfileImportReview{}, launcherTransferError("capacity")
	}
	return LauncherProfileImportReview{Digest: launcherTransferDigest([]byte(document)), Revision: launcherTransferRevision(s), Name: p.Name, ItemCount: len(p.Items), WidgetCount: len(p.Widgets), Notices: []string{"selectionsRequireRepair", "widgetsDisabled", "iconsNotIncluded"}}, nil
}

func (a *App) ImportLauncherProfile(document, digest, expectedRevision string) (LauncherProfileImportResult, error) {
	p, err := config.ParseLauncherProfile([]byte(document))
	if err != nil {
		return LauncherProfileImportResult{}, launcherTransferError("invalidDocument")
	}
	if digest != launcherTransferDigest([]byte(document)) {
		return LauncherProfileImportResult{}, launcherTransferError("staleDigest")
	}
	a.viewMu.Lock()
	a.syncLauncherItemAdmissionLocked()
	m := a.launcherItems
	if !a.launcherChoicesAllowedLocked() || m == nil || m.closed {
		a.viewMu.Unlock()
		return LauncherProfileImportResult{}, launcherTransferError("unavailable")
	}
	epoch, session := m.epoch, a.sessionGeneration
	a.viewMu.Unlock()
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	s := a.settingsSnapshot()
	if expectedRevision != launcherTransferRevision(s.ReplacementDock) {
		return LauncherProfileImportResult{}, launcherTransferError("staleRevision")
	}
	if len(s.ReplacementDock.Profiles) >= 8 {
		return LauncherProfileImportResult{}, launcherTransferError("capacity")
	}
	var seed [16]byte
	if _, err = rand.Read(seed[:]); err != nil {
		return LauncherProfileImportResult{}, launcherTransferError("unavailable")
	}
	fresh := hex.EncodeToString(seed[:])
	p.ID = "import-" + fresh
	for i := range p.Items {
		item := &p.Items[i]
		if item.Kind == "app" || item.Kind == "file" || item.Kind == "folder" {
			item.ReferenceID = "selection-" + launcherTransferDigest([]byte(fresh + ":" + item.ID))[:32]
		}
	}
	s.ReplacementDock.Profiles = append(s.ReplacementDock.Profiles, p)
	if err = config.ValidateReplacementDock(s.ReplacementDock); err != nil {
		return LauncherProfileImportResult{}, launcherTransferError("invalidDocument")
	}
	// Validate the exact pretty-printed representation used by SaveFile, not
	// just the smaller compact RPC fallback: nested Dock decoding is bounded.
	var persisted bytes.Buffer
	if err = config.Save(&persisted, s); err != nil {
		return LauncherProfileImportResult{}, launcherTransferError("invalidDocument")
	}
	if _, err = config.Load(bytes.NewReader(persisted.Bytes())); err != nil {
		return LauncherProfileImportResult{}, launcherTransferError("capacity")
	}
	b, err := json.Marshal(s)
	if err != nil {
		return LauncherProfileImportResult{}, launcherTransferError("invalidDocument")
	}
	a.viewMu.Lock()
	a.syncLauncherItemAdmissionLocked()
	admitted := a.launcherChoicesAllowedLocked() && a.launcherItems == m && !m.closed && m.ctx.Err() == nil && m.epoch == epoch && a.sessionGeneration == session
	a.viewMu.Unlock()
	if !admitted {
		return LauncherProfileImportResult{}, launcherTransferError("unavailable")
	}
	if err = a.saveSettingsLocked(s); err != nil {
		return LauncherProfileImportResult{}, launcherTransferError("saveFailed")
	}
	return LauncherProfileImportResult{ProfileID: p.ID, SettingsJSON: string(b)}, nil
}
