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

func TestAddEditCancelAdvanceDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subscriptions.json")
	m := send(t, New(path, nil), tea.WindowSizeMsg{Width: 100, Height: 36})
	if !strings.Contains(m.View(), "Noch keine Abos") {
		t.Fatalf("empty view:\n%s", m.View())
	}

	m = send(t, m, keyRunes("a"))
	if m.screen != screenForm {
		t.Fatal("expected form")
	}
	m = send(t, m,
		keyRunes("Netflix"),
		key(tea.KeyEnter),
		keyRunes("12,99"),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
		keyRunes("15.11.2026"),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
		keyRunes("14"),
		key(tea.KeyEnter),
		key(tea.KeyRight),
		key(tea.KeyEnter),
		keyRunes("Familienabo"),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
	)
	if m.screen != screenList {
		t.Fatalf("screen %d err %q\n%s", m.screen, m.form.err, m.View())
	}
	if m.status != "Gespeichert: Netflix" {
		t.Fatalf("status %q", m.status)
	}
	if len(m.subs) != 1 {
		t.Fatalf("subs %+v", m.subs)
	}
	got := m.subs[0]
	if got.Name != "Netflix" || got.AmountCents != 1299 || got.Interval != sub.IntervalMonth {
		t.Fatalf("%+v", got)
	}
	if got.Category != "Streaming" || got.NoticeDays != 14 || got.Note != "Familienabo" {
		t.Fatalf("%+v", got)
	}
	if got.ChargeMode != sub.ChargeManual {
		t.Fatalf("charge mode %q", got.ChargeMode)
	}
	if got.NextBilling.Year() != 2026 || got.NextBilling.Month() != time.November || got.NextBilling.Day() != 15 {
		t.Fatalf("date %v", got.NextBilling)
	}
	view := m.View()
	if !strings.Contains(view, "Netflix") || !strings.Contains(view, "12,99 €") || !strings.Contains(view, "Familienabo") {
		t.Fatalf("view:\n%s", view)
	}

	m = send(t, m, keyRunes("e"), key(tea.KeyDown))
	for range 5 {
		m = send(t, m, key(tea.KeyBackspace))
	}
	m = send(t, m, keyRunes("8,50"))
	for range 7 {
		m = send(t, m, key(tea.KeyDown))
	}
	m = send(t, m, key(tea.KeyEnter))
	if m.screen != screenList {
		t.Fatalf("edit screen %d err %q\n%s", m.screen, m.form.err, m.View())
	}
	if m.subs[0].AmountCents != 850 {
		t.Fatalf("amount %d value %q", m.subs[0].AmountCents, "edited")
	}
	if !strings.Contains(m.View(), "8,50 €") {
		t.Fatalf("view:\n%s", m.View())
	}

	m = send(t, m, keyRunes("c"))
	if m.subs[0].Status != sub.StatusCancelled {
		t.Fatal("expected cancelled")
	}
	if !strings.Contains(m.View(), "gekündigt") {
		t.Fatalf("view:\n%s", m.View())
	}
	if sub.Summarize(m.subs, time.Now()).MonthlyCents != 0 {
		t.Fatal("cancelled should not count")
	}

	m = send(t, m, keyRunes("f"))
	if !strings.Contains(m.View(), "Keine aktiven Abos") {
		t.Fatalf("view:\n%s", m.View())
	}
	m = send(t, m, keyRunes("f"), keyRunes("c"))
	if m.subs[0].Status != sub.StatusActive {
		t.Fatal("expected active again")
	}

	m = send(t, m, keyRunes("n"))
	if m.subs[0].NextBilling.Month() != time.December || m.subs[0].NextBilling.Day() != 15 {
		t.Fatalf("advanced %v", m.subs[0].NextBilling)
	}
	if !strings.Contains(m.status, "15.12.2026") {
		t.Fatalf("status %q", m.status)
	}
	if len(m.subs[0].Charges) != 1 || m.subs[0].Charges[0].AmountCents != 850 {
		t.Fatalf("charges %+v", m.subs[0].Charges)
	}

	m = send(t, m, keyRunes("d"))
	if m.screen != screenConfirm {
		t.Fatal("expected confirm")
	}
	m = send(t, m, keyRunes("j"))
	if len(m.subs) != 0 || m.screen != screenList {
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
	path := filepath.Join(t.TempDir(), "subscriptions.json")
	m := send(t, New(path, nil), tea.WindowSizeMsg{Width: 100, Height: 36}, keyRunes("a"))
	for range 9 {
		m = send(t, m, key(tea.KeyEnter))
	}
	if m.screen != screenForm {
		t.Fatal("expected to stay on form")
	}
	if m.form.err == "" {
		t.Fatal("expected validation error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should not exist")
	}
}

func TestHelpAndQuit(t *testing.T) {
	m := New(filepath.Join(t.TempDir(), "s.json"), nil)
	m = send(t, m, keyRunes("?"))
	if m.screen != screenHelp || !strings.Contains(m.View(), "Hilfe") {
		t.Fatalf("view:\n%s", m.View())
	}
	m = send(t, m, key(tea.KeyEsc))
	if m.screen != screenList {
		t.Fatal("expected list")
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
	path := filepath.Join(t.TempDir(), "subscriptions.json")
	m := send(t, New(path, nil), tea.WindowSizeMsg{Width: 100, Height: 36}, keyRunes("a"))
	m = send(t, m,
		keyRunes("Adobe"),
		key(tea.KeyEnter),
		keyRunes("239,88"),
		key(tea.KeyEnter),
		key(tea.KeyRight),
		key(tea.KeyRight),
		key(tea.KeyEnter),
		keyRunes("01.01.2027"),
		key(tea.KeyEnter),
		key(tea.KeyRight),
		key(tea.KeyRight),
		key(tea.KeyRight),
		key(tea.KeyRight),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
	)
	if m.screen != screenList {
		t.Fatalf("err %q\n%s", m.form.err, m.View())
	}
	if m.subs[0].Interval != sub.IntervalYear || m.subs[0].Category != "Software" {
		t.Fatalf("%+v", m.subs[0])
	}
	if m.subs[0].MonthlyCents() != 1999 {
		t.Fatalf("monthly %d", m.subs[0].MonthlyCents())
	}
	if !strings.Contains(m.View(), "239,88 €") || !strings.Contains(m.View(), "≈ 19,99 €/Monat") {
		t.Fatalf("view:\n%s", m.View())
	}
}

func TestAutoPriceHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subscriptions.json")
	m := send(t, New(path, nil), tea.WindowSizeMsg{Width: 110, Height: 40}, keyRunes("a"))
	m = send(t, m,
		keyRunes("Netflix"),
		key(tea.KeyEnter),
		keyRunes("12,99"),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
		keyRunes("15.12.2026"),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
	)
	if m.screen != screenList {
		t.Fatalf("err %q\n%s", m.form.err, m.View())
	}
	if m.subs[0].ChargeMode != sub.ChargeAuto {
		t.Fatalf("mode %q", m.subs[0].ChargeMode)
	}
	billing := m.subs[0].NextBilling
	m = send(t, m, keyRunes("n"))
	if !m.subs[0].NextBilling.Equal(billing) {
		t.Fatalf("auto advanced %v", m.subs[0].NextBilling)
	}
	if !strings.Contains(m.status, "Automatisch") {
		t.Fatalf("status %q", m.status)
	}

	m = send(t, m, keyRunes("p"),
		keyRunes("15,99"),
		key(tea.KeyEnter),
		keyRunes("03.2027"),
		key(tea.KeyEnter),
		key(tea.KeyEnter),
	)
	if m.screen != screenList {
		t.Fatalf("price err %q\n%s", m.form.err, m.View())
	}
	if m.subs[0].AmountCents != 1299 {
		t.Fatalf("current %d", m.subs[0].AmountCents)
	}
	view := m.View()
	if !strings.Contains(view, "ab 03.2027") || !strings.Contains(view, "15,99 €") {
		t.Fatalf("view:\n%s", view)
	}
	m = send(t, m, keyRunes("h"))
	if m.screen != screenHistory {
		t.Fatal("expected history")
	}
	history := m.View()
	if !strings.Contains(history, "12,99 €") || !strings.Contains(history, "15,99 €") || !strings.Contains(history, "03.2027") || !strings.Contains(history, "+23,1 %") {
		t.Fatalf("history:\n%s", history)
	}
	if !strings.Contains(history, "Noch keine Abbuchungen") {
		t.Fatalf("history:\n%s", history)
	}

	loaded, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || len(loaded[0].Prices) != 2 {
		t.Fatalf("%+v", loaded)
	}
}
