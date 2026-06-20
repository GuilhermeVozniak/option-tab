package main

import "testing"

type mockGreeter struct{ called string }

func (m *mockGreeter) Greet(name string) string {
	m.called = name
	return "mocked:" + name
}

func TestApp_Greet_DelegatesToGreeter(t *testing.T) {
	mg := &mockGreeter{}
	app := &App{greeter: mg}

	got := app.Greet("Gui")

	if got != "mocked:Gui" {
		t.Errorf("Greet() = %q, want %q", got, "mocked:Gui")
	}
	if mg.called != "Gui" {
		t.Errorf("greeter received %q, want %q", mg.called, "Gui")
	}
}
