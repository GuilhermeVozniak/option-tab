//go:build !darwin

package platform

import (
	"context"
	"errors"
)

type (
	unsupportedBattery struct{}
	unsupportedNetwork struct{}
)

func NewBatterySource() BatterySource { return unsupportedBattery{} }
func NewNetworkSource() NetworkSource { return unsupportedNetwork{} }
func (unsupportedBattery) ObserveBattery(context.Context, func(BatterySnapshot)) error {
	return errors.New("battery status unsupported")
}

func (unsupportedNetwork) ObserveNetwork(context.Context, bool, func(NetworkSnapshot)) error {
	return errors.New("network status unsupported")
}
