package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"abo-tracker/internal/store"
	"abo-tracker/internal/tui"
)

func main() {
	data := flag.String("data", "", "JSON-Datei (Standard: im Konfigurationsordner)")
	flag.Parse()

	path, err := resolvePath(*data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Datenpfad: %v\n", err)
		os.Exit(1)
	}
	subs, err := store.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Daten konnten nicht gelesen werden: %v\nDatei: %s\n", err, path)
		os.Exit(1)
	}

	p := tea.NewProgram(tui.New(path, subs), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Programmfehler: %v\n", err)
		os.Exit(1)
	}
}

func resolvePath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "abo-tracker", "subscriptions.json"), nil
}
