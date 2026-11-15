package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"abo-tracker/internal/store"
	"abo-tracker/internal/sub"
)

func send(t *testing.T, m *Model, msgs ...tea.Msg) *Model {
	t.Helper()
	var cur tea.Model = m
	for _, msg := range msgs {
		next, _ := cur.Update(msg)
		cur = next
	}
	got, ok := cur.(*Model)
	if !ok {
		t.Fatalf("model type %T", cur)
	}
	return got
}

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func key(k tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: k}
}

func inDays(n int) time.Time {
	return sub.DateOnly(time.Now()).AddDate(0, 0, n)
}

func newModel(t *testing.T, subs []sub.Subscription) (*Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "subscriptions.json")
	return send(t, New(path, subs), tea.WindowSizeMsg{Width: 120, Height: 36}), path
}

func TestAddEditCancelAdvanceDelete(t *testing.T) {
	m, path := newModel(t, nil)
	if !strings.Contains(m.View(), "noch keine Abos") {
		t.Fatalf("empty view:\n%s", m.View())
	}

	date := inDays(10)
	m = send(t, m, keyRunes("n"))
	if m.mode != modeInsert || !strings.Contains(m.View(), "NEUES ABO") {
		t.Fatalf("expected form\n%s", m.View())
	}
	m = send(t, m,
		keyRunes("Netflix"), key(tea.KeyTab),
		keyRunes("12,99"), key(tea.KeyTab),
		key(tea.KeyTab),
		keyRunes(sub.FormatDate(date)), key(tea.KeyTab),
		key(tea.KeyTab),
		keyRunes("14"), key(tea.KeyTab),
		key(tea.KeyRight), key(tea.KeyTab),
		keyRunes("Familienabo"),
		key(tea.KeyEnter),
	)
	if m.mode != modeNormal {
		t.Fatalf("mode %d err %q\n%s", m.mode, m.form.err, m.View())
	}
	if m.flash != "Gespeichert: Netflix" {
		t.Fatalf("flash %q", m.flash)
	}
	got := m.subs[0]
	if got.Name != "Netflix" || got.AmountCents != 1299 || got.Interval != sub.IntervalMonth {
		t.Fatalf("%+v", got)
	}
	if got.Category != "Streaming" || got.NoticeDays != 14 || got.Note != "Familienabo" || got.ChargeMode != sub.ChargeManual {
		t.Fatalf("%+v", got)
	}
	if !got.NextBilling.Equal(date) {
		t.Fatalf("date %v", got.NextBilling)
	}
	view := m.View()
	for _, want := range []string{"Netflix", "12,99 €", "Familienabo", "DETAIL", "ÜBERSICHT", "NORMAL"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}

	m = send(t, m, keyRunes("e"), key(tea.KeyTab))
	for range 5 {
		m = send(t, m, key(tea.KeyBackspace))
	}
	m = send(t, m, keyRunes("8,50"), key(tea.KeyEnter))
	if m.mode != modeNormal || m.subs[0].AmountCents != 850 {
		t.Fatalf("edit err %q amount %d", m.form.err, m.subs[0].AmountCents)
	}

	m = send(t, m, keyRunes("x"))
	if m.subs[0].Status != sub.StatusCancelled || !strings.Contains(m.View(), "GEKÜNDIGT") {
		t.Fatalf("expected cancelled\n%s", m.View())
	}
	if sub.Summarize(m.subs, time.Now()).MonthlyCents != 0 {
		t.Fatal("cancelled should not count")
	}

	m = send(t, m, keyRunes("H"))
	if !strings.Contains(m.View(), "keine aktiven Abos") {
		t.Fatalf("view:\n%s", m.View())
	}
	m = send(t, m, keyRunes("H"), keyRunes("x"))
	if m.subs[0].Status != sub.StatusActive {
		t.Fatal("expected active again")
	}

	m = send(t, m, keyRunes("r"))
	want := sub.Advance(date, sub.IntervalMonth)
	if !m.subs[0].NextBilling.Equal(want) {
		t.Fatalf("advanced %v want %v", m.subs[0].NextBilling, want)
	}
	if len(m.subs[0].Charges) != 1 || m.subs[0].Charges[0].AmountCents != 850 {
		t.Fatalf("charges %+v", m.subs[0].Charges)
	}

	m = send(t, m, keyRunes("d"))
	if m.chord != "d" || !strings.Contains(m.View(), "d…") {
		t.Fatalf("chord %q", m.chord)
	}
	m = send(t, m, keyRunes("d"))
	if m.mode != modeConfirm {
		t.Fatal("expected confirm")
	}
	m = send(t, m, keyRunes("y"))
	if len(m.subs) != 0 || m.mode != modeNormal {
		t.Fatalf("after delete %+v", m.subs)
	}
	loaded, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Fatalf("file %+v", loaded)
	}
}

func TestEmptySaveStaysOnForm(t *testing.T) {
	m, path := newModel(t, nil)
	m = send(t, m, keyRunes("n"), key(tea.KeyEnter))
	if m.mode != modeInsert || m.form.err == "" {
		t.Fatalf("mode %d err %q", m.mode, m.form.err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should not exist")
	}
	m = send(t, m, key(tea.KeyEsc))
	if m.mode != modeNormal {
		t.Fatal("esc should close the form")
	}
}

func TestHelpAndQuit(t *testing.T) {
	m, _ := newModel(t, nil)
	m = send(t, m, keyRunes("?"))
	if m.mode != modeHelp || !strings.Contains(m.View(), "NAVIGATION") {
		t.Fatalf("view:\n%s", m.View())
	}
	m = send(t, m, key(tea.KeyEsc))
	if m.mode != modeNormal {
		t.Fatal("expected normal")
	}
	_, cmd := m.Update(keyRunes("q"))
	if cmd == nil {
		t.Fatal("expected quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("cmd result %T", cmd())
	}
}

func TestIntervalSelect(t *testing.T) {
	m, _ := newModel(t, nil)
	m = send(t, m, keyRunes("n"),
		keyRunes("Adobe"), key(tea.KeyTab),
		keyRunes("239,88"), key(tea.KeyTab),
		key(tea.KeyRight), key(tea.KeyRight), key(tea.KeyTab),
		keyRunes(sub.FormatDate(inDays(60))), key(tea.KeyTab),
		key(tea.KeyRight), key(tea.KeyRight), key(tea.KeyRight), key(tea.KeyRight),
		key(tea.KeyEnter),
	)
	if m.mode != modeNormal {
		t.Fatalf("err %q\n%s", m.form.err, m.View())
	}
	if m.subs[0].Interval != sub.IntervalYear || m.subs[0].Category != "Software" {
		t.Fatalf("%+v", m.subs[0])
	}
	if m.subs[0].MonthlyCents() != 1999 {
		t.Fatalf("monthly %d", m.subs[0].MonthlyCents())
	}
	if !strings.Contains(m.View(), "239,88 €") || !strings.Contains(m.View(), "19,99 €") {
		t.Fatalf("view:\n%s", m.View())
	}
}

func TestAutoPriceHistory(t *testing.T) {
	m, path := newModel(t, nil)
	m = send(t, m, keyRunes("n"),
		keyRunes("Netflix"), key(tea.KeyTab),
		keyRunes("12,99"), key(tea.KeyTab),
		key(tea.KeyTab),
		keyRunes(sub.FormatDate(inDays(20))),
		key(tea.KeyEnter),
	)
	if m.mode != modeNormal {
		t.Fatalf("err %q\n%s", m.form.err, m.View())
	}
	if m.subs[0].ChargeMode != sub.ChargeAuto {
		t.Fatalf("mode %q", m.subs[0].ChargeMode)
	}
	billing := m.subs[0].NextBilling
	m = send(t, m, keyRunes("r"))
	if !m.subs[0].NextBilling.Equal(billing) || !strings.Contains(m.flash, "Automatisch") {
		t.Fatalf("auto advanced %v flash %q", m.subs[0].NextBilling, m.flash)
	}

	month := sub.FormatMonth(time.Now().AddDate(0, 4, 0))
	m = send(t, m, keyRunes("p"), keyRunes("15,99"), key(tea.KeyTab), keyRunes(month), key(tea.KeyEnter))
	if m.mode != modeNormal {
		t.Fatalf("price err %q\n%s", m.form.err, m.View())
	}
	if m.subs[0].AmountCents != 1299 {
		t.Fatalf("current %d", m.subs[0].AmountCents)
	}
	view := m.View()
	if !strings.Contains(view, month+" 15,99 €") || !strings.Contains(view, "+23,1 %") {
		t.Fatalf("view:\n%s", view)
	}

	m = send(t, m, keyRunes("o"))
	if m.mode != modeHistory {
		t.Fatal("expected history")
	}
	history := m.View()
	for _, want := range []string{"12,99 €", "15,99 €", month, "+23,1 %", "Noch keine Abbuchungen"} {
		if !strings.Contains(history, want) {
			t.Fatalf("missing %q:\n%s", want, history)
		}
	}
	m = send(t, m, key(tea.KeyEsc))
	if m.mode != modeNormal {
		t.Fatal("esc should close history")
	}

	loaded, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || len(loaded[0].Prices) != 2 {
		t.Fatalf("%+v", loaded)
	}
}

func TestCommandPaletteAndNavigation(t *testing.T) {
	subs := []sub.Subscription{
		{ID: 1, Name: "Netflix", AmountCents: 1299, Interval: sub.IntervalMonth, NextBilling: inDays(5), Category: "Streaming", Status: sub.StatusActive},
		{ID: 2, Name: "Spotify", AmountCents: 1099, Interval: sub.IntervalMonth, NextBilling: inDays(9), Category: "Streaming", Status: sub.StatusActive},
		{ID: 3, Name: "Gym", AmountCents: 2990, Interval: sub.IntervalMonth, NextBilling: inDays(12), Category: "Fitness", Status: sub.StatusCancelled},
	}
	m, _ := newModel(t, subs)

	m = send(t, m, keyRunes("G"))
	if m.cursor != 2 {
		t.Fatalf("G cursor %d", m.cursor)
	}
	m = send(t, m, keyRunes("g"), keyRunes("g"))
	if m.cursor != 0 {
		t.Fatalf("gg cursor %d", m.cursor)
	}
	m = send(t, m, keyRunes("j"))
	if m.cursor != 1 {
		t.Fatalf("j cursor %d", m.cursor)
	}

	m = send(t, m, keyRunes(":"))
	if m.mode != modeCommand || !strings.Contains(m.View(), "neues Abo anlegen") {
		t.Fatalf("palette:\n%s", m.View())
	}
	m = send(t, m, keyRunes("verl"))
	if !strings.Contains(m.View(), "1 Treffer") {
		t.Fatalf("filter:\n%s", m.View())
	}
	m = send(t, m, key(tea.KeyEnter))
	if m.mode != modeHistory || m.historyID != 2 {
		t.Fatalf("mode %d history %d", m.mode, m.historyID)
	}

	m = send(t, m, key(tea.KeyEsc), key(tea.KeyCtrlP))
	if m.mode != modeCommand {
		t.Fatal("ctrl+p should open the palette")
	}
	m = send(t, m, key(tea.KeyEsc), keyRunes("]"))
	if strings.Contains(m.View(), "DETAIL") {
		t.Fatalf("detail should be hidden:\n%s", m.View())
	}
	m = send(t, m, keyRunes("["))
	if strings.Contains(m.View(), "ÜBERSICHT") {
		t.Fatalf("overview should be hidden:\n%s", m.View())
	}
}
