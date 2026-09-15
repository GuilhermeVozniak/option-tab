package main

import "option-tab/internal/widgets"

// Localize presentation copies only after the catalog's exact built-in digest
// check. Manifest bytes, package identity, settings values and grants stay intact.
func localizeBuiltinWidgetCatalog(item *WidgetCatalogItem) {
	text := func(en, pt, es string) widgets.Localized {
		return widgets.Localized{"en": en, "pt-BR": pt, "es": es}
	}
	switch item.PackageID {
	case "org.optiontab.clock":
		item.Name = text("Clock", "Relógio", "Reloj")
		item.Description = text("Built-in local clock widget.", "Widget integrado de relógio local.", "Widget integrado de reloj local.")
		for i := range item.Settings {
			switch item.Settings[i].ID {
			case "timezone":
				item.Settings[i].Name = text("Time zone", "Fuso horário", "Zona horaria")
			case "format":
				item.Settings[i].Name = text("Time format", "Formato da hora", "Formato de hora")
			}
		}
	case "org.optiontab.battery":
		item.Name = text("Battery", "Bateria", "Batería")
		item.Description = text("Built-in local battery widget.", "Widget integrado de bateria local.", "Widget integrado de batería local.")
	case "org.optiontab.network":
		item.Name = text("Network", "Rede", "Red")
		item.Description = text("Built-in local network widget.", "Widget integrado de rede local.", "Widget integrado de red local.")
	case "org.optiontab.audio":
		item.Name = text("Audio", "Áudio", "Audio")
		item.Description = text("Built-in local audio widget.", "Widget integrado de áudio local.", "Widget integrado de audio local.")
	}
}
