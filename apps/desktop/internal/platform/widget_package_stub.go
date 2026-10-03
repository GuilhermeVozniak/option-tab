//go:build !darwin

package platform

func NewWidgetPackageSource() WidgetPackageSource { return newWidgetPackageSource(nil) }
