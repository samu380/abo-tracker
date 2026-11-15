package tui

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"abo-tracker/internal/store"
	"abo-tracker/internal/sub"
)

type mode int

const (
	modeNormal mode = iota
	modeInsert
	modeConfirm
	modeHelp
	modeHistory
	modeCommand
)

type flashKind int

const (
	flashInfo flashKind = iota
	flashOK
	flashErr
)

const (
	leftPaneW  = 26
	rightPaneW = 34
	minBodyW   = 40
)

type Model struct {
	path          string
	subs          []sub.Subscription
	cursor        int
	scroll        int
	activeOnly    bool
	hideLeft      bool
	hideRight     bool
	mode          mode
	width         int
	height        int
	flash         string
	flashKind     flashKind
	chord         string
	form          formState
	pending       sub.Subscription
	historyID     int
	historyOffset int
	query         string
	paletteCursor int
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
	m := &Model{path: path, subs: cp}
	if changed && path != "" {
		if err := m.persist(cp); err != nil {
			m.setFlash(flashErr, "Speichern fehlgeschlagen: "+err.Error())
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
		m.clampCursor()
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		switch m.mode {
		case modeInsert:
			return m.updateInsert(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeHelp:
			return m.updateHelp(msg)
		case modeHistory:
			return m.updateHistory(msg)
		case modeCommand:
			return m.updateCommand(msg)
		default:
			return m.updateNormal(msg)
		}
	}
	if m.mode == modeInsert {
		return m, m.updateActiveInput(msg)
	}
	return m, nil
}

func (m *Model) updateNormal(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := key.String()
	m.flash = ""
	if m.chord != "" {
		seq := m.chord + k
		m.chord = ""
		if a, ok := chordKeys[seq]; ok {
			return m.apply(a)
		}
		if k == "esc" {
			return m, nil
		}
	}
	if a, ok := normalKeys[k]; ok {
		return m.apply(a)
	}
	if isChordLeader(k) {
		m.chord = k
	}
	return m, nil
}

func (m *Model) apply(a action) (tea.Model, tea.Cmd) {
	vis := m.visibleSubs()
	m.clampCursor()
	current, hasCurrent := selected(vis, m.cursor)
	needCurrent := func() bool {
		if !hasCurrent {
			m.setFlash(flashErr, "Noch kein Abo ausgewählt")
		}
		return hasCurrent
	}

	switch a {
	case actQuit:
		return m, tea.Quit
	case actDown:
		m.cursor++
	case actUp:
		m.cursor--
	case actTop:
		m.cursor = 0
	case actBottom:
		m.cursor = len(vis) - 1
	case actHalfDown:
		m.cursor += m.halfPage()
	case actHalfUp:
		m.cursor -= m.halfPage()
	case actNew:
		return m.openForm(buildForm(nil))
	case actEdit:
		if needCurrent() {
			return m.openForm(buildForm(&current))
		}
	case actToggleCancel:
		if needCurrent() {
			m.toggleCancel(current)
		}
	case actReschedule:
		if needCurrent() {
			m.advanceBilling(current)
		}
	case actPrice:
		if needCurrent() {
			return m.openForm(buildPriceForm(current))
		}
	case actHistory:
		if needCurrent() {
			m.historyID = current.ID
			m.historyOffset = 0
			m.mode = modeHistory
		}
	case actDelete:
		if needCurrent() {
			m.pending = current
			m.mode = modeConfirm
		}
	case actToggleCancelled:
		m.activeOnly = !m.activeOnly
		if hasCurrent {
			m.selectID(current.ID)
		}
		if m.activeOnly {
			m.setFlash(flashInfo, "nur aktive Abos")
		} else {
			m.setFlash(flashInfo, "alle Abos")
		}
	case actToggleLeft:
		m.hideLeft = !m.hideLeft
	case actToggleRight:
		m.hideRight = !m.hideRight
	case actPalette:
		m.query = ""
		m.paletteCursor = 0
		m.mode = modeCommand
	case actHelp:
		m.mode = modeHelp
	}
	m.clampCursor()
	return m, nil
}

func (m *Model) updateCommand(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	hits := paletteHits(m.query)
	switch key.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case "enter":
		if len(hits) == 0 {
			return m, nil
		}
		a := hits[clampInt(m.paletteCursor, 0, len(hits)-1)].act
		m.mode = modeNormal
		return m.apply(a)
	case "up", "ctrl+p", "ctrl+k":
		m.paletteCursor = clampInt(m.paletteCursor-1, 0, len(hits)-1)
		return m, nil
	case "down", "ctrl+n", "ctrl+j", "tab":
		m.paletteCursor = clampInt(m.paletteCursor+1, 0, len(hits)-1)
		return m, nil
	case "backspace":
		r := []rune(m.query)
		if len(r) > 0 {
			m.query = string(r[:len(r)-1])
		}
		m.paletteCursor = 0
		return m, nil
	}
	switch key.Type {
	case tea.KeyRunes:
		m.query += string(key.Runes)
		m.paletteCursor = 0
	case tea.KeySpace:
		m.query += " "
		m.paletteCursor = 0
	}
	return m, nil
}

func (m *Model) updateHelp(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "?", "q":
		m.mode = modeNormal
	}
	return m, nil
}

func (m *Model) updateHistory(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "o", "q":
		m.mode = modeNormal
	case "up", "k":
		m.historyOffset--
	case "down", "j":
		m.historyOffset++
	}
	m.clampHistory()
	return m, nil
}

func (m *Model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "n", "N":
		m.mode = modeNormal
	case "y", "Y", "j", "J", "enter":
		m.mode = modeNormal
		name := m.pending.Name
		if err := m.persist(withoutID(m.subs, m.pending.ID)); err != nil {
			m.setFlash(flashErr, "Löschen fehlgeschlagen: "+err.Error())
			return m, nil
		}
		m.clampCursor()
		m.setFlash(flashOK, "gelöscht: "+name)
	}
	return m, nil
}

func (m *Model) updateInsert(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.form.err = ""
	n := len(m.form.fields)
	switch key.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case "enter":
		return m.submitForm()
	case "tab", "down":
		m.form.index = (m.form.index + 1) % n
		return m, m.focusForm()
	case "shift+tab", "up":
		m.form.index = (m.form.index - 1 + n) % n
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
	m.mode = modeNormal
	m.form = formState{}
	m.selectID(s.ID)
	m.setFlash(flashOK, "Gespeichert: "+s.Name)
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
	m.mode = modeNormal
	m.form = formState{}
	m.selectID(updated.ID)
	m.setFlash(flashOK, "ab "+sub.FormatMonth(month)+": "+sub.FormatEUR(amount))
	return m, nil
}

func (m *Model) openForm(f formState) (tea.Model, tea.Cmd) {
	m.mode = modeInsert
	m.form = f
	m.applyInputWidth()
	return m, m.focusForm()
}

func (m *Model) toggleCancel(s sub.Subscription) {
	if s.Status == sub.StatusActive {
		s.Status = sub.StatusCancelled
	} else {
		s.Status = sub.StatusActive
	}
	if err := m.persist(upsert(m.subs, s)); err != nil {
		m.setFlash(flashErr, "Speichern fehlgeschlagen: "+err.Error())
		return
	}
	m.selectID(s.ID)
	if s.Status == sub.StatusActive {
		m.setFlash(flashOK, "wieder aktiv: "+s.Name)
	} else {
		m.setFlash(flashOK, "gekündigt: "+s.Name)
	}
}

func (m *Model) advanceBilling(s sub.Subscription) {
	if s.Status != sub.StatusActive {
		m.setFlash(flashErr, "gekündigt: erst mit x reaktivieren")
		return
	}
	if s.ChargeMode.OrAuto() == sub.ChargeAuto && sub.DaysBetween(time.Now(), s.NextBilling) >= 0 {
		m.setFlash(flashInfo, "Automatisch: der Termin rückt von selbst weiter")
		return
	}
	s = s.RecordChargeAndAdvance()
	if err := m.persist(upsert(m.subs, s)); err != nil {
		m.setFlash(flashErr, "Speichern fehlgeschlagen: "+err.Error())
		return
	}
	msg := "nächster Termin: " + sub.FormatDate(s.NextBilling)
	if sub.DaysBetween(time.Now(), s.NextBilling) < 0 {
		msg += " · weiterhin überfällig"
	}
	m.selectID(s.ID)
	m.setFlash(flashOK, msg)
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
	if f.kind != kindSelect || len(f.options) == 0 {
		return false
	}
	n := len(f.options)
	f.option = (f.option + delta + n) % n
	return true
}

func (m *Model) applyInputWidth() {
	w := m.formWidth() - 2 - 4 - 20 - 1
	if w < 8 {
		w = 8
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
	for i, s := range m.visibleSubs() {
		if s.ID == id {
			m.cursor = i
			break
		}
	}
	m.clampCursor()
}

func (m *Model) clampCursor() {
	m.cursor = clampInt(m.cursor, 0, len(m.visibleSubs())-1)
}

func (m *Model) clampHistory() {
	s, ok := sub.Find(m.subs, m.historyID)
	if !ok {
		m.historyOffset = 0
		return
	}
	m.historyOffset = clampInt(m.historyOffset, 0, len(historyBodyLines(s))-m.historyPage())
}

func (m *Model) size() (int, int) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 30
	}
	return w, h
}

func (m *Model) listHeight() int {
	_, h := m.size()
	return max(1, h-3)
}

func (m *Model) halfPage() int {
	return max(1, m.listHeight()/2)
}

func (m *Model) historyPage() int {
	_, h := m.size()
	return max(3, h-1-2-2-5)
}

func (m *Model) formWidth() int {
	w, _ := m.size()
	return clampInt(w-4, 30, 64)
}

func (m *Model) panes() (bool, bool) {
	w, _ := m.size()
	left, right := !m.hideLeft, !m.hideRight
	if left && right && w < leftPaneW+rightPaneW+minBodyW {
		left = false
	}
	if right && w < rightPaneW+minBodyW {
		right = false
	}
	if left && w < leftPaneW+minBodyW {
		left = false
	}
	return left, right
}

func (m *Model) setFlash(kind flashKind, text string) {
	m.flashKind = kind
	m.flash = text
}

func (m *Model) tooSmall() bool {
	return m.width > 0 && m.height > 0 && (m.width < 50 || m.height < 14)
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
