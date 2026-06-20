// Package greeter holds an example business unit. Logic lives behind the
// Greeter interface so the Wails binding layer depends on behavior, not a
// concrete type, and can be tested with a mock.
package greeter

import (
	"fmt"
	"strings"
)

// Greeter produces a greeting for a name.
type Greeter interface {
	Greet(name string) string
}

// DefaultGreeter is the production implementation.
type DefaultGreeter struct{}

// New returns a production Greeter.
func New() *DefaultGreeter { return &DefaultGreeter{} }

// Greet returns a friendly greeting, defaulting to "World" when empty.
func (g *DefaultGreeter) Greet(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "World"
	}
	return fmt.Sprintf("Hello, %s!", name)
}
