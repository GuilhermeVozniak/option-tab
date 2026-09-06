//go:build !darwin

package platform

import "option-tab/internal/domain"

func (p *stub) AppWindowPresence(domain.AppID) WindowPresence { return WindowsUnknown }
