package main

import (
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type LauncherItemPanelState struct {
	Session       uint64                      `json:"session"`
	Revision      uint64                      `json:"revision"`
	ParentEpoch   uint64                      `json:"parentEpoch"`
	ParentSession uint64                      `json:"parentSession"`
	DisplayUUID   string                      `json:"displayUUID"`
	ProfileID     string                      `json:"profileID"`
	ItemID        string                      `json:"itemID"`
	Kind          string                      `json:"kind"`
	Title         string                      `json:"title"`
	Open          bool                        `json:"open"`
	Bounds        domain.Bounds               `json:"bounds"`
	Folder        *LauncherItemFolderState    `json:"folder,omitempty"`
	Windows       *AutomationPreviewViewState `json:"windows,omitempty"`
	Error         string                      `json:"error,omitempty"`
}
type LauncherItemFolderState struct {
	FolderIdentity string                 `json:"folderIdentity"`
	Status         string                 `json:"status"`
	Reason         string                 `json:"reason"`
	View           string                 `json:"view"`
	Entries        []platform.FolderEntry `json:"entries"`
	Sort           platform.FolderSort    `json:"sort"`
	Partial        bool                   `json:"partial"`
	Revision       uint64                 `json:"revision"`
}

func cloneLauncherItemPanelState(s LauncherItemPanelState) LauncherItemPanelState {
	if s.Folder != nil {
		f := *s.Folder
		f.Entries = append([]platform.FolderEntry{}, f.Entries...)
		s.Folder = &f
	}
	s.Windows = cloneLauncherWindowState(s.Windows)
	return s
}
