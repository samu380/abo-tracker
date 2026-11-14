package tui

import (
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"abo-tracker/internal/store"
	"abo-tracker/internal/sub"
)

type screen int

const (
	screenList screen = iota
	screenForm
	screenConfirm
	screenHelp
	screenHistory
)

type statusKind int

const (
	statusInfo statusKind = iota
	statusOK
	statusErr
)

type Model struct {
	path          string
	subs          []sub.Subscription
	cursor        int
	offset        int
	activeOnly    bool
	screen        screen
	width         int
	height        int
	status        string
	statusKind    statusKind
	form          formState
	pending       sub.Subscription
	historyID     int
	historyOffset int
}

func New(path string, subs []sub.Subscription) *Model {
	cp := make([]sub.Subscription, len(subs))
	copy(cp, subs)
	today := time.Now()
	changed := false
	for i := range cp {
		var did bool
		cp[i], did = cp[i].RollForward(today)
		changed = changed || did
	}
	m := &Model{
		path:   path,
		subs:   cp,
		screen: screenList,
	}
	if changed && path != "" {
		if err := m.persist(cp); err != nil {
			m.setStatus(statusErr, "Speichern fehlgeschlagen: "+err.Error())
		}
	}
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyInputWidth()
		m.clampCursor(len(m.visibleSubs()))
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.screen {
		case screenForm:
			return m.updateForm(msg)
		case screenConfirm:
			return m.updateConfirm(msg)
		case screenHelp:
			return m.updateHelp(msg)
		case screenHistory:
			return m.updateHistory(msg)
		default:
			return m.updateList(msg)
		}
	}
	if m.screen == screenForm {
		return m, m.updateActiveInput(msg)
	}
	return m, nil
}

func (m *Model) updateList(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	m.statusKind = statusInfo
	vis := m.visibleSubs()
	m.clampCursor(len(vis))

	switch key.String() {
	case "q":
		return m, tea.Quit
	case "?":
		m.screen = screenHelp
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		m.clampCursor(len(vis))
		return m, nil
	case "down", "j":
		if m.cursor < len(vis)-1 {
			m.cursor++
		}
		m.clampCursor(len(vis))
		return m, nil
	case "pgup":
		m.cursor -= m.pageSize()
		m.clampCursor(len(vis))
		return m, nil
	case "pgdown":
		m.cursor += m.pageSize()
		m.clampCursor(len(vis))
		return m, nil
	case "home", "g":
		m.cursor = 0
		m.clampCursor(len(vis))
		return m, nil
	case "end", "G":
		m.cursor = len(vis) - 1
		m.clampCursor(len(vis))
		return m, nil
	case "a":
		return m.openForm(nil)
	case "e":
		s, ok := selected(vis, m.cursor)
		if !ok {
			m.setStatus(statusErr, "Noch kein Abo ausgewählt")
			return m, nil
		}
		return m.openForm(&s)
	case "f":
		var keep int
		if s, ok := selected(vis, m.cursor); ok {
			keep = s.ID
		}
		m.activeOnly = !m.activeOnly
		m.cursor = 0
		m.offset = 0
		if keep != 0 {
			m.selectID(keep)
		} else {
			m.clampCursor(len(m.visibleSubs()))
		}
		if m.activeOnly {
			m.setStatus(statusInfo, "Nur aktive Abos")
		} else {
			m.setStatus(statusInfo, "Alle Abos")
		}
		return m, nil
	case "c":
		return m.toggleCancel(vis)
	case "n":
		return m.advanceBilling(vis)
	case "p":
		s, ok := selected(vis, m.cursor)
		if !ok {
			m.setStatus(statusErr, "Noch kein Abo ausgewählt")
			return m, nil
		}
		return m.openPriceForm(s)
	case "h":
		s, ok := selected(vis, m.cursor)
		if !ok {
			m.setStatus(statusErr, "Noch kein Abo ausgewählt")
			return m, nil
		}
		m.historyID = s.ID
		m.historyOffset = 0
		m.screen = screenHistory
		return m, nil
	case "d":
		s, ok := selected(vis, m.cursor)
		if !ok {
			m.setStatus(statusErr, "Noch kein Abo ausgewählt")
			return m, nil
		}
		m.pending = s
		m.screen = screenConfirm
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) updateHistory(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "h", "q":
		m.screen = screenList
	case "up", "k":
		if m.historyOffset > 0 {
			m.historyOffset--
		}
	case "down", "j":
		m.historyOffset++
		m.clampHistory()
	}
	return m, nil
}

func (m *Model) clampHistory() {
	s, ok := sub.Find(m.subs, m.historyID)
	if !ok {
		m.historyOffset = 0
		return
	}
	maxOff := len(historyBodyLines(s)) - m.historyPage()
	if maxOff < 0 {
		maxOff = 0
	}
	if m.historyOffset > maxOff {
		m.historyOffset = maxOff
	}
	if m.historyOffset < 0 {
		m.historyOffset = 0
	}
}

func (m *Model) historyPage() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	h -= 10
	if h < 4 {
		return 4
	}
	return h
}

func (m *Model) updateHelp(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "?", "q":
		m.screen = screenList
	}
	return m, nil
}

func (m *Model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "n", "N":
		m.screen = screenList
		return m, nil
	case "j", "J", "y", "Y", "enter":
		name := m.pending.Name
		next := withoutID(m.subs, m.pending.ID)
		if err := m.persist(next); err != nil {
			m.screen = screenList
			m.setStatus(statusErr, "Löschen fehlgeschlagen: "+err.Error())
			return m, nil
		}
		m.screen = screenList
		m.clampCursor(len(m.visibleSubs()))
		m.setStatus(statusOK, "Gelöscht: "+name)
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) updateForm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.form.err = ""
	switch key.String() {
	case "esc":
		m.screen = screenList
		return m, nil
	case "up":
		if m.form.index > 0 {
			m.form.index--
		}
		return m, m.focusForm()
	case "down":
		if m.form.index < len(m.form.fields)-1 {
			m.form.index++
		}
		return m, m.focusForm()
	case "enter":
		if m.form.fields[m.form.index].kind == kindAction {
			return m.submitForm()
		}
		if m.form.index < len(m.form.fields)-1 {
			m.form.index++
		}
		return m, m.focusForm()
	case "left":
		if m.nudgeSelect(-1) {
			return m, nil
		}
	case "right":
		if m.nudgeSelect(1) {
			return m, nil
		}
	}
	return m, m.updateActiveInput(key)
}

func (m *Model) submitForm() (tea.Model, tea.Cmd) {
	if m.form.kind == formPrice {
		return m.submitPrice()
	}
	s, err := m.form.toSubscription(m.subs)
	if err != nil {
		var fe fieldError
		if errors.As(err, &fe) && fe.field >= 0 && fe.field < len(m.form.fields) {
			m.form.index = fe.field
		}
		m.form.err = err.Error()
		return m, m.focusForm()
	}
	if err := m.persist(upsert(m.subs, s)); err != nil {
		m.form.err = "Speichern fehlgeschlagen: " + err.Error()
		return m, nil
	}
	m.screen = screenList
	m.form = formState{}
	m.selectID(s.ID)
	m.setStatus(statusOK, "Gespeichert: "+s.Name)
	return m, nil
}

func (m *Model) submitPrice() (tea.Model, tea.Cmd) {
	amount, err := sub.ParseAmount(m.form.fields[priceFieldAmount].input.Value())
	if err != nil {
		m.form.index = priceFieldAmount
		m.form.err = err.Error()
		return m, m.focusForm()
	}
	month, err := sub.ParseMonth(m.form.fields[priceFieldMonth].input.Value())
	if err != nil {
		m.form.index = priceFieldMonth
		m.form.err = err.Error()
		return m, m.focusForm()
	}
	prev, ok := sub.Find(m.subs, m.form.id)
	if !ok {
		m.form.err = "Abo wurde nicht gefunden"
		return m, nil
	}
	updated := prev.SetPriceFrom(month, amount, time.Now())
	if err := updated.Validate(); err != nil {
		m.form.err = err.Error()
		return m, nil
	}
	if err := m.persist(upsert(m.subs, updated)); err != nil {
		m.form.err = "Speichern fehlgeschlagen: " + err.Error()
		return m, nil
	}
	m.screen = screenList
	m.form = formState{}
	m.selectID(updated.ID)
	m.setStatus(statusOK, "Ab "+sub.FormatMonth(month)+": "+sub.FormatEUR(amount))
	return m, nil
}

func (m *Model) openPriceForm(existing sub.Subscription) (tea.Model, tea.Cmd) {
	m.screen = screenForm
	m.form = buildPriceForm(existing)
	m.applyInputWidth()
	return m, m.focusForm()
}

func (m *Model) openForm(existing *sub.Subscription) (tea.Model, tea.Cmd) {
	m.screen = screenForm
	m.form = buildForm(existing)
	m.applyInputWidth()
	return m, m.focusForm()
}

func (m *Model) toggleCancel(vis []sub.Subscription) (tea.Model, tea.Cmd) {
	s, ok := selected(vis, m.cursor)
	if !ok {
		m.setStatus(statusErr, "Noch kein Abo ausgewählt")
		return m, nil
	}
	if s.Status == sub.StatusActive {
		s.Status = sub.StatusCancelled
	} else {
		s.Status = sub.StatusActive
	}
	if err := m.persist(upsert(m.subs, s)); err != nil {
		m.setStatus(statusErr, "Speichern fehlgeschlagen: "+err.Error())
		return m, nil
	}
	msg := "Gekündigt: " + s.Name
	if s.Status == sub.StatusActive {
		msg = "Wieder aktiv: " + s.Name
	}
	m.selectID(s.ID)
	m.setStatus(statusOK, msg)
	return m, nil
}

func (m *Model) advanceBilling(vis []sub.Subscription) (tea.Model, tea.Cmd) {
	s, ok := selected(vis, m.cursor)
	if !ok {
		m.setStatus(statusErr, "Noch kein Abo ausgewählt")
		return m, nil
	}
	if s.Status != sub.StatusActive {
		m.setStatus(statusErr, "Gekündigt: erst mit c reaktivieren")
		return m, nil
	}
	if s.ChargeMode.OrAuto() == sub.ChargeAuto && sub.DaysBetween(time.Now(), s.NextBilling) >= 0 {
		m.setStatus(statusInfo, "Automatisch: der Termin rückt von selbst weiter")
		return m, nil
	}
	s = s.RecordChargeAndAdvance()
	if err := m.persist(upsert(m.subs, s)); err != nil {
		m.setStatus(statusErr, "Speichern fehlgeschlagen: "+err.Error())
		return m, nil
	}
	msg := "Nächster Termin: " + sub.FormatDate(s.NextBilling)
	if sub.DaysBetween(time.Now(), s.NextBilling) < 0 {
		msg += " · weiterhin überfällig"
	}
	m.selectID(s.ID)
	m.setStatus(statusOK, msg)
	return m, nil
}

func (m *Model) focusForm() tea.Cmd {
	var cmd tea.Cmd
	for i := range m.form.fields {
		if i == m.form.index && m.form.fields[i].kind == kindText {
			cmd = m.form.fields[i].input.Focus()
		} else {
			m.form.fields[i].input.Blur()
		}
	}
	return cmd
}

func (m *Model) updateActiveInput(msg tea.Msg) tea.Cmd {
	idx := m.form.index
	if idx < 0 || idx >= len(m.form.fields) || m.form.fields[idx].kind != kindText {
		return nil
	}
	var cmd tea.Cmd
	m.form.fields[idx].input, cmd = m.form.fields[idx].input.Update(msg)
	return cmd
}

func (m *Model) nudgeSelect(delta int) bool {
	if m.form.index < 0 || m.form.index >= len(m.form.fields) {
		return false
	}
	f := &m.form.fields[m.form.index]
	if f.kind != kindSelect {
		return false
	}
	next := f.option + delta
	if next < 0 || next >= len(f.options) {
		return true
	}
	f.option = next
	return true
}

func (m *Model) applyInputWidth() {
	w := m.contentWidth() - 24
	if w > 42 {
		w = 42
	}
	if w < 16 {
		w = 16
	}
	for i := range m.form.fields {
		m.form.fields[i].input.Width = w
	}
}

func (m *Model) persist(next []sub.Subscription) error {
	if err := store.Save(m.path, next); err != nil {
		return err
	}
	m.subs = next
	return nil
}

func (m *Model) visibleSubs() []sub.Subscription {
	return sub.Visible(m.subs, m.activeOnly)
}

func (m *Model) selectID(id int) {
	vis := m.visibleSubs()
	m.cursor = 0
	for i, s := range vis {
		if s.ID == id {
			m.cursor = i
			break
		}
	}
	m.clampCursor(len(vis))
}

func (m *Model) clampCursor(n int) {
	if n <= 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	page := m.pageSize()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+page {
		m.offset = m.cursor - page + 1
	}
	maxOff := n - page
	if maxOff < 0 {
		maxOff = 0
	}
	if m.offset > maxOff {
		m.offset = maxOff
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *Model) pageSize() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	h -= 2
	reserved := 11
	if m.status != "" {
		reserved++
	}
	n := (h - reserved) / 2
	if n < 1 {
		return 1
	}
	return n
}

func (m *Model) contentWidth() int {
	w := m.width
	if w <= 0 {
		w = 80
	}
	w -= 4
	if w < 20 {
		w = 20
	}
	return w
}

func (m *Model) setStatus(kind statusKind, text string) {
	m.statusKind = kind
	m.status = text
}

func (m *Model) tooSmall() bool {
	return m.width > 0 && m.height > 0 && (m.width < 56 || m.height < 20)
}

func selected(vis []sub.Subscription, cursor int) (sub.Subscription, bool) {
	if cursor < 0 || cursor >= len(vis) {
		return sub.Subscription{}, false
	}
	return vis[cursor], true
}

func upsert(subs []sub.Subscription, s sub.Subscription) []sub.Subscription {
	out := make([]sub.Subscription, len(subs))
	copy(out, subs)
	for i := range out {
		if out[i].ID == s.ID {
			out[i] = s
			return out
		}
	}
	return append(out, s)
}

func withoutID(subs []sub.Subscription, id int) []sub.Subscription {
	out := make([]sub.Subscription, 0, len(subs))
	for _, s := range subs {
		if s.ID != id {
			out = append(out, s)
		}
	}
	return out
}

func (m *Model) helpList(s *sub.Subscription) string {
	cancel := "c kündigen"
	if s != nil && s.Status == sub.StatusCancelled {
		cancel = "c reaktivieren"
	}
	filter := "f nur aktive"
	if m.activeOnly {
		filter = "f alle"
	}
	line1 := fmt.Sprintf("↑↓ wählen    a neu    e bearbeiten    %s", cancel)
	line2 := "n Termin    p Preis    h Verlauf    d löschen"
	line3 := fmt.Sprintf("%s    ? Hilfe    q beenden", filter)
	return helpStyle.Render(line1) + "\n" + helpStyle.Render(line2) + "\n" + helpStyle.Render(line3)
}
