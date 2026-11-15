package tui

import "strings"

// action ist alles, was man auslösen kann, unabhängig von der Taste.
// Tastenbelegung und Befehlspalette rufen beide apply mit einer action auf.
type action int

const (
	actQuit action = iota
	actDown
	actUp
	actTop
	actBottom
	actHalfDown
	actHalfUp
	actNew
	actEdit
	actToggleCancel
	actReschedule
	actPrice
	actHistory
	actDelete
	actToggleCancelled
	actToggleLeft
	actToggleRight
	actPalette
	actHelp
)

var normalKeys = map[string]action{
	"j":      actDown,
	"down":   actDown,
	"k":      actUp,
	"up":     actUp,
	"G":      actBottom,
	"end":    actBottom,
	"home":   actTop,
	"ctrl+d": actHalfDown,
	"pgdown": actHalfDown,
	"ctrl+u": actHalfUp,
	"pgup":   actHalfUp,
	"n":      actNew,
	"e":      actEdit,
	"x":      actToggleCancel,
	"r":      actReschedule,
	"p":      actPrice,
	"o":      actHistory,
	"H":      actToggleCancelled,
	"[":      actToggleLeft,
	"]":      actToggleRight,
	":":      actPalette,
	"ctrl+p": actPalette,
	"?":      actHelp,
	"q":      actQuit,
}

// chordKeys sind Zwei-Tasten-Folgen. Die erste Taste wird gemerkt und in der
// Statuszeile als "g…" angezeigt.
var chordKeys = map[string]action{
	"gg": actTop,
	"dd": actDelete,
}

func isChordLeader(k string) bool {
	for c := range chordKeys {
		if strings.HasPrefix(c, k) {
			return true
		}
	}
	return false
}

type paletteEntry struct {
	act   action
	label string
	keys  string
}

var paletteEntries = []paletteEntry{
	{actNew, "neues Abo anlegen", "n"},
	{actEdit, "Abo bearbeiten", "e"},
	{actToggleCancel, "kündigen oder reaktivieren", "x"},
	{actReschedule, "manuellen Termin weiterschieben", "r"},
	{actPrice, "neuer Preis ab Monat", "p"},
	{actHistory, "Verlauf öffnen", "o"},
	{actDelete, "Abo löschen", "dd"},
	{actToggleCancelled, "gekündigte ein- oder ausblenden", "H"},
	{actToggleLeft, "Übersicht ein- oder ausblenden", "["},
	{actToggleRight, "Detail ein- oder ausblenden", "]"},
	{actTop, "zum ersten Abo", "gg"},
	{actBottom, "zum letzten Abo", "G"},
	{actHelp, "Hilfe", "?"},
	{actQuit, "beenden", "q"},
}

func paletteHits(query string) []paletteEntry {
	var out []paletteEntry
	for _, e := range paletteEntries {
		if fuzzyMatch(e.label, query) {
			out = append(out, e)
		}
	}
	return out
}

func fuzzyMatch(label, query string) bool {
	q := []rune(strings.ToLower(strings.TrimSpace(query)))
	if len(q) == 0 {
		return true
	}
	j := 0
	for _, r := range strings.ToLower(label) {
		if j < len(q) && r == q[j] {
			j++
		}
	}
	return j == len(q)
}
