package main

import (
	"context"
	"option-tab/internal/greeter"
)

// App is the Wails-bound layer: a thin adapter that holds service
// interfaces and delegates to them. It contains no business logic.
type App struct {
	ctx     context.Context
	greeter greeter.Greeter
}

// NewApp wires production dependencies.
func NewApp() *App {
	return &App{greeter: greeter.New()}
}

// startup captures the Wails runtime context.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet is exposed to the frontend through Wails bindings.
func (a *App) Greet(name string) string {
	return a.greeter.Greet(name)
}
