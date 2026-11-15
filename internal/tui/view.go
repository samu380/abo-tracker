package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"abo-tracker/internal/sub"
)

func (m *Model) View() string {
	if m.tooSmall() {
		return "Fenster zu klein.\nMindestens 50×14.\n\nctrl+c beendet.\n"
	}
	w, h := m.size()
	bodyH := h - 1
	today := time.Now()
	showLeft, showRight := m.panes()
	lw, rw := 0, 0
	if showLeft {
		lw = leftPaneW
	}
	if showRight {
		rw = rightPaneW
	}
	cw := w - lw - rw

	var left, right []string
	if showLeft {
		left = m.viewOverview(lw, bodyH, today)
	}
	center := m.viewCenter(cw, bodyH, today)
	if showRight {
		right = m.viewDetail(rw, bodyH, today)
	}
	rows := make([]string, bodyH)
	for i := range rows {
		var b strings.Builder
		if showLeft {
			b.WriteString(left[i])
		}
		b.WriteString(center[i])
		if showRight {
			b.WriteString(right[i])
		}
		rows[i] = b.String()
	}

	switch m.mode {
	case modeInsert:
		rows = overlay(rows, m.viewForm(), w)
	case modeConfirm:
		rows = overlay(rows, m.viewConfirm(w), w)
	case modeHelp:
		rows = overlay(rows, m.viewHelp(w, bodyH), w)
	case modeHistory:
		rows = overlay(rows, m.viewHistory(w), w)
	case modeCommand:
		rows = overlay(rows, m.viewPalette(w, bodyH), w)
	}
	rows = append(rows, m.viewStatus(w, today))
	return strings.Join(rows, "\n")
}

// --- linke Spalte ---------------------------------------------------------

func (m *Model) viewOverview(w, h int, today time.Time) []string {
	sum := sub.Summarize(m.subs, today)
	overdue := stFg
	if sum.OverdueCents > 0 {
		overdue = stRed
	}
	all := len(m.subs)
	lines := []string{
		line(w, cPanel, sg(" ÜBERSICHT", stDimB)),
		blank(w, cPanel),
		line(w, cPanel, sg(" KOSTEN", stAccB)),
		kv(w, "Monatlich", sub.FormatEUR(sum.MonthlyCents), stGreen),
		kv(w, "30 Tage", sub.FormatEUR(sum.UpcomingCents), stFg),
		kv(w, "Überfällig", sub.FormatEUR(sum.OverdueCents), overdue),
		blank(w, cPanel),
		line(w, cPanel, sg(" ABOS", lipgloss.NewStyle().Foreground(cTeal).Bold(true))),
		kv(w, "aktiv", fmt.Sprint(sum.Active), stDim),
		kv(w, "gekündigt", fmt.Sprint(sum.Cancelled), stDim),
		blank(w, cPanel),
		line(w, cPanel, sg(" ANSICHT", lipgloss.NewStyle().Foreground(cCopper).Bold(true))),
		filterRow(w, "alle", all, !m.activeOnly),
		filterRow(w, "nur aktive", sum.Active, m.activeOnly),
	}
	return fill(lines, h, w, cPanel)
}

func kv(w int, label, value string, valueSt lipgloss.Style) string {
	inner := w - 4
	gap := inner - lipgloss.Width(label) - lipgloss.Width(value)
	if gap < 1 {
		gap = 1
	}
	return line(w, cPanel, sg("   "+label+strings.Repeat(" ", gap), stFg), sg(value, valueSt))
}

func filterRow(w int, label string, count int, active bool) string {
	bg := cPanel
	prefix := "   "
	if active {
		bg = cSelected
		prefix = " ▸ "
	}
	n := fmt.Sprint(count)
	gap := w - 1 - lipgloss.Width(prefix) - lipgloss.Width(label) - len(n)
	if gap < 1 {
		gap = 1
	}
	return line(w, bg, sg(prefix, stAccent), sg(label+strings.Repeat(" ", gap), stFg), sg(n, stDim))
}

// --- mittlere Spalte ------------------------------------------------------

func (m *Model) viewCenter(w, h int, today time.Time) []string {
	vis := m.visibleSubs()
	view := "alle"
	if m.activeOnly {
		view = "aktiv"
	}
	header := line(w, cPanel,
		sg(" € ", stAccB),
		sg(shortPath(m.path), stBold),
		sg("  •  ", stDim),
		sg(fmt.Sprintf("%d Abos", len(vis)), stDim),
		sg("  •  ", stDim),
		sg("ansicht:"+view, stAccent),
	)
	out := []string{header, blank(w, cBg)}
	listH := h - 2

	if len(m.subs) == 0 {
		return append(out, m.viewEmpty(w, listH)...)
	}

	var lines []string
	cursorLine := -1
	if len(vis) == 0 {
		lines = append(lines, line(w, cBg, sg("   keine aktiven Abos · H zeigt auch gekündigte", stDim)))
	} else {
		priceW := 0
		for _, s := range vis {
			priceW = max(priceW, lipgloss.Width(priceLabel(s, today)))
		}
		counts := map[sub.Status]int{}
		for _, s := range vis {
			counts[s.Status]++
		}
		var last sub.Status
		for i, s := range vis {
			if i == 0 || s.Status != last {
				if i > 0 {
					lines = append(lines, blank(w, cBg))
				}
				lines = append(lines, groupHeader(w, s.Status, counts[s.Status]))
				last = s.Status
			}
			if i == m.cursor {
				cursorLine = len(lines)
			}
			lines = append(lines, listRow(w, i, s, i == m.cursor, priceW, today))
		}
	}

	m.scroll = keepCursorVisible(m.scroll, cursorLine, listH, len(lines))
	end := min(len(lines), m.scroll+listH)
	out = append(out, lines[m.scroll:end]...)
	return fill(out, h, w, cBg)
}

func groupHeader(w int, status sub.Status, count int) string {
	label, st := "AKTIV", lipgloss.NewStyle().Foreground(cGreen).Bold(true)
	if status == sub.StatusCancelled {
		label, st = "GEKÜNDIGT", stDimB
	}
	return line(w, cBg,
		sg(" ", stFg),
		sg(fmt.Sprintf("(%d)", count), stDim),
		sg("  ", stFg),
		sg(label, st),
		sg("  ", stFg),
		sg(strings.Repeat("─", max(0, w)), stBorder),
	)
}

func listRow(w, i int, s sub.Subscription, isCursor bool, priceW int, today time.Time) string {
	bg := cBg
	if isCursor {
		bg = cCursor
	}
	numSt := stDim
	if isCursor {
		numSt = stFg
	}
	glyph, glyphSt := "  ", stAccent
	switch {
	case s.Status == sub.StatusCancelled:
		glyph, glyphSt = "✗ ", stDone
	case isCursor:
		glyph = "▸ "
	}

	att := s.AttentionAt(today)
	nameSt := stFg
	if isCursor {
		nameSt = stBold
	}
	switch {
	case s.Status == sub.StatusCancelled:
		nameSt = stDone
	case att == sub.AttentionOverdue || att == sub.AttentionNoticeMissed:
		nameSt = stRed
	}

	date, dateSt := dateLabel(s, today)
	const dateW = 10
	fixed := 5 + 2 + 2 + priceW + 2 + dateW + 1
	nameW := max(4, w-fixed)
	return line(w, bg,
		sg(fmt.Sprintf(" %3d ", i+1), numSt),
		sg(glyph, glyphSt),
		sg(padRight(truncate(s.Name, nameW), nameW), nameSt),
		sg("  ", stFg),
		sg(padLeft(priceLabel(s, today), priceW), stFg),
		sg("  ", stFg),
		sg(padRight(date, dateW), dateSt),
	)
}

func priceLabel(s sub.Subscription, today time.Time) string {
	return sub.FormatEUR(s.PriceOn(today)) + "/" + s.Interval.Per()
}

func dateLabel(s sub.Subscription, today time.Time) (string, lipgloss.Style) {
	if s.Status == sub.StatusCancelled {
		return "gekündigt", stDone
	}
	days := sub.DaysBetween(today, s.NextBilling)
	att := s.AttentionAt(today)
	switch {
	case days < 0:
		return "überfällig", stRed
	case days == 0:
		return "heute", stAmber
	case days == 1:
		return "morgen", stAmber
	}
	st := stDim
	switch att {
	case sub.AttentionNoticeMissed:
		st = stRed
	case sub.AttentionNotice, sub.AttentionSoon:
		st = stAmber
	}
	return sub.FormatDate(s.NextBilling), st
}

func (m *Model) viewEmpty(w, h int) []string {
	bw := min(56, w-2)
	body := []string{
		blank(bw-2, cPanel),
		line(bw-2, cPanel, sg("  noch keine Abos · los geht's", stBold)),
		blank(bw-2, cPanel),
		hintRow(bw-2, "n", "Abo anlegen"),
		hintRow(bw-2, "?", "alle Tasten"),
		hintRow(bw-2, ":", "Befehlspalette"),
		hintRow(bw-2, "q", "beenden"),
		blank(bw-2, cPanel),
		line(bw-2, cPanel, sg("  Beispiel: Netflix, 12,99 € monatlich", stDim)),
		blank(bw-2, cPanel),
	}
	win := box(bw, []seg{sg("abo-tracker", stAccB)}, body)
	rows := fill(nil, h, w, cBg)
	return overlay(rows, win, w)
}

func hintRow(w int, key, desc string) string {
	return line(w, cPanel, sg("   "+padRight(key, 4), stAccB), sg(desc, stFg))
}

// --- rechte Spalte --------------------------------------------------------

func (m *Model) viewDetail(w, h int, today time.Time) []string {
	lines := []string{line(w, cPanel, sg(" DETAIL", stDimB)), blank(w, cPanel)}
	s, ok := selected(m.visibleSubs(), m.cursor)
	if !ok {
		lines = append(lines, line(w, cPanel, sg(" (kein Abo)", stDim)))
		return fill(lines, h, w, cPanel)
	}
	row := func(label string, segs ...seg) {
		lines = append(lines, line(w, cPanel, append([]seg{sg(" "+padRight(label, 10), stDim)}, segs...)...))
	}
	price := s.PriceOn(today)
	row("name", sg(s.Name, stBold))
	row("betrag", sg(sub.FormatEUR(price)+" / "+s.Interval.Per(), stFg))
	if s.Interval != sub.IntervalMonth {
		row("≈ monat", sg(sub.FormatEUR(sub.MonthlyEquivalent(price, s.Interval)), stFg))
	}
	row("intervall", sg(s.Interval.Label(), stFg))

	row("nächste", sg(sub.FormatDate(s.NextBilling), stFg))
	if s.Status == sub.StatusActive {
		row("", relativeDays(sub.DaysBetween(today, s.NextBilling), "überfällig"))
	}
	if s.NoticeDays > 0 {
		row("frist", sg(sub.FormatDate(sub.NoticeDate(s)), stFg))
		if s.Status == sub.StatusActive {
			row("", relativeDays(sub.DaysBetween(today, sub.NoticeDate(s)), "abgelaufen"))
		}
	} else {
		row("frist", sg("—", stDim))
	}
	row("abbuchung", sg(strings.ToLower(s.ChargeMode.OrAuto().Label()), stFg))
	row("kategorie", sg(s.Category, lipgloss.NewStyle().Foreground(cCopper)))
	if s.Status == sub.StatusActive {
		row("status", sg("aktiv", stGreen))
	} else {
		row("status", sg("gekündigt", stDone))
	}
	if up, ok := s.UpcomingPrice(today); ok {
		row("neu ab", sg(sub.FormatMonth(up.From)+" "+sub.FormatEUR(up.AmountCents), stAmber),
			sg(" "+sub.FormatPercentChange(price, up.AmountCents), stDim))
	}
	if s.Note != "" {
		row("notiz", sg(s.Note, stFg))
	}

	lines = append(lines, blank(w, cPanel), line(w, cPanel, sg(" VERLAUF", stDimB)), blank(w, cPanel))
	charges := s.ChargesNewestFirst()
	row("preise", sg(fmt.Sprint(max(1, len(s.Prices))), stFg))
	if len(charges) > 0 {
		row("letzte", sg(sub.FormatDate(charges[0].Date)+" · "+sub.FormatEUR(charges[0].AmountCents), stFg))
	}
	lines = append(lines, line(w, cPanel, sg(" o", stAccB), sg(" öffnet den Verlauf", stDim)))
	return fill(lines, h, w, cPanel)
}

func relativeDays(days int, pastLabel string) seg {
	switch {
	case days < 0:
		if days == -1 {
			return sg("seit gestern "+pastLabel, stRed)
		}
		return sg(fmt.Sprintf("seit %d Tagen %s", -days, pastLabel), stRed)
	case days == 0:
		return sg("heute", stAmber)
	case days == 1:
		return sg("morgen", stAmber)
	case days <= 14:
		return sg(fmt.Sprintf("in %d Tagen", days), stAmber)
	default:
		return sg(fmt.Sprintf("in %d Tagen", days), stDim)
	}
}

// --- Statuszeile ----------------------------------------------------------

func (m *Model) modeLabel() string {
	switch m.mode {
	case modeInsert:
		return "INSERT"
	case modeConfirm:
		return "LÖSCHEN"
	case modeHelp:
		return "HILFE"
	case modeHistory:
		return "VERLAUF"
	case modeCommand:
		return "BEFEHL"
	default:
		return "NORMAL"
	}
}

func (m *Model) modeHint() string {
	switch m.mode {
	case modeInsert:
		return "Tab/↑↓ Feld · ←→ Auswahl · Enter speichern · Esc abbrechen"
	case modeConfirm:
		return "y löschen · n/Esc abbrechen"
	case modeHelp:
		return "? Hilfe schließen"
	case modeHistory:
		return "j/k blättern · Esc schließen"
	case modeCommand:
		return "tippen zum Filtern · Enter ausführen · Esc abbrechen"
	default:
		return "j/k · n neu · e bearbeiten · x kündigen · r Termin · : Befehle · ? Hilfe · q beenden"
	}
}

func (m *Model) viewStatus(w int, today time.Time) string {
	label := m.modeLabel()
	if m.chord != "" {
		label += " " + m.chord + "…"
	}
	chipBg := cModeBg
	if m.flash != "" {
		label += " · " + m.flash
		if m.flashKind == flashErr {
			chipBg = cRed
		}
	}
	chip := " " + label + " "
	sum := sub.Summarize(m.subs, today)
	right := fmt.Sprintf("%d aktiv · %s · abo-tracker ", sum.Active, sub.FormatDate(today))
	mid := w - lipgloss.Width(chip) - lipgloss.Width(right)
	if mid < 4 {
		right = ""
		mid = w - lipgloss.Width(chip)
	}
	hint := ""
	if mid > 2 {
		hint = padRight(truncate("  "+m.modeHint(), mid-1), mid)
	}
	return line(w, cStatusBar,
		sg(chip, lipgloss.NewStyle().Background(chipBg).Foreground(cModeFg).Bold(true)),
		sg(hint, lipgloss.NewStyle().Foreground(cStatusFg)),
		sg(right, stDim),
	)
}

// --- Fenster ---------------------------------------------------------------

func (m *Model) viewForm() []string {
	bw := m.formWidth()
	inner := bw - 2
	const labelW = 20
	body := []string{blank(inner, cPanel)}
	for i, f := range m.form.fields {
		active := i == m.form.index
		marker, labelSt := "    ", stDim
		if active {
			marker, labelSt = "  › ", stAccB
		}
		var value seg
		switch f.kind {
		case kindSelect:
			text := "‹ " + f.options[clampInt(f.option, 0, len(f.options)-1)] + " ›"
			st := stFg
			if active {
				st = stAccB
			}
			value = sg(text, st)
		default:
			value = rawSeg(f.input.View())
		}
		body = append(body, line(inner, cPanel, sg(marker, stAccent), sg(padRight(f.label, labelW), labelSt), value))
	}
	body = append(body, blank(inner, cPanel))
	if m.form.err != "" {
		body = append(body, line(inner, cPanel, sg("  "+m.form.err, stRed)))
	} else {
		body = append(body, blank(inner, cPanel))
	}
	body = append(body, line(inner, cPanel, sg("  Enter speichern · Esc abbrechen", stDim)))
	return box(bw, []seg{sg(m.form.title, stAccB)}, body)
}

func (m *Model) viewConfirm(w int) []string {
	bw := clampInt(w-4, 30, 52)
	inner := bw - 2
	p := m.pending
	body := []string{
		blank(inner, cPanel),
		line(inner, cPanel, sg("  "+p.Name, stAccB)),
		line(inner, cPanel, sg("  "+sub.FormatEUR(p.AmountCents)+" / "+p.Interval.Per(), stDim)),
		blank(inner, cPanel),
		line(inner, cPanel, sg("  Der Eintrag wird aus der Datei entfernt.", stFg)),
		blank(inner, cPanel),
		line(inner, cPanel, sg("  y", stAccB), sg(" löschen   ", stFg), sg("n", stAccB), sg(" abbrechen", stFg)),
		blank(inner, cPanel),
	}
	return box(bw, []seg{sg("abo", stAccB), sg(" · löschen", stDim)}, body)
}

func (m *Model) viewHistory(w int) []string {
	bw := clampInt(w-4, 34, 60)
	inner := bw - 2
	s, ok := sub.Find(m.subs, m.historyID)
	if !ok {
		return box(bw, []seg{sg("abo", stAccB), sg(" · verlauf", stDim)},
			[]string{blank(inner, cPanel), line(inner, cPanel, sg("  Abo wurde nicht gefunden.", stFg)), blank(inner, cPanel)})
	}
	m.clampHistory()
	all := historyBodyLines(s)
	end := min(len(all), m.historyOffset+m.historyPage())
	body := []string{
		blank(inner, cPanel),
		line(inner, cPanel, sg("  "+s.ChargeMode.OrAuto().Label(), stDim)),
		blank(inner, cPanel),
	}
	for _, l := range all[m.historyOffset:end] {
		st := stFg
		if l == "Preise" || l == "Abbuchungen" {
			st = stAccB
		}
		body = append(body, line(inner, cPanel, sg("  "+l, st)))
	}
	body = append(body, blank(inner, cPanel), line(inner, cPanel, sg("  j/k blättern · Esc schließen", stDim)))
	return box(bw, []seg{sg("abo", stAccB), sg(" · verlauf · "+s.Name, stDim)}, body)
}

func historyBodyLines(s sub.Subscription) []string {
	lines := []string{"Preise"}
	prices := orderedPrices(s)
	if len(prices) == 0 {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "seit Beginn", sub.FormatEUR(s.AmountCents)))
	}
	for i, p := range prices {
		var span string
		if p.From.IsZero() {
			span = "seit Beginn"
		} else {
			span = "ab " + sub.FormatMonth(p.From)
		}
		if i+1 < len(prices) && !prices[i+1].From.IsZero() {
			prev := prices[i+1].From.AddDate(0, -1, 0)
			if p.From.IsZero() {
				span = "bis " + sub.FormatMonth(prev)
			} else {
				span = sub.FormatMonth(p.From) + " – " + sub.FormatMonth(prev)
			}
		}
		amount := sub.FormatEUR(p.AmountCents)
		if i > 0 {
			amount += "   " + sub.FormatPercentChange(prices[i-1].AmountCents, p.AmountCents)
		}
		lines = append(lines, fmt.Sprintf("  %-22s %s", span, amount))
	}
	lines = append(lines, "", "Abbuchungen")
	charges := s.ChargesNewestFirst()
	if len(charges) == 0 {
		return append(lines, "  Noch keine Abbuchungen.")
	}
	for _, c := range charges {
		lines = append(lines, fmt.Sprintf("  %-22s %s", sub.FormatDate(c.Date), sub.FormatEUR(c.AmountCents)))
	}
	return lines
}

func orderedPrices(s sub.Subscription) []sub.Price {
	prices := append([]sub.Price(nil), s.Prices...)
	sort.SliceStable(prices, func(i, j int) bool {
		a, b := prices[i].From, prices[j].From
		if a.IsZero() {
			return !b.IsZero()
		}
		if b.IsZero() {
			return false
		}
		return a.Before(b)
	})
	return prices
}

type helpEntry struct{ keys, desc string }

type helpSection struct {
	title   string
	entries []helpEntry
}

var helpLeft = []helpSection{
	{"NAVIGATION", []helpEntry{
		{"j / k  (↓ / ↑)", "nächstes / voriges Abo"},
		{"gg / G", "erstes / letztes Abo"},
		{"Ctrl-d / Ctrl-u", "halbe Seite runter / hoch"},
	}},
	{"BEARBEITEN", []helpEntry{
		{"n", "neues Abo"},
		{"e", "bearbeiten"},
		{"x", "kündigen / reaktivieren"},
		{"r", "manuellen Termin weiter"},
		{"p", "neuer Preis ab Monat"},
		{"dd", "löschen"},
	}},
	{"FORMULAR", []helpEntry{
		{"Tab / ↑↓", "Feld wechseln"},
		{"← / →", "Auswahl ändern"},
		{"Enter / Esc", "speichern / abbrechen"},
	}},
}

var helpRight = []helpSection{
	{"ANSICHT", []helpEntry{
		{"o", "Verlauf öffnen"},
		{"H", "gekündigte ein / aus"},
		{"[ / ]", "Übersicht / Detail"},
	}},
	{"SYSTEM", []helpEntry{
		{": / Ctrl-P", "Befehlspalette"},
		{"?", "Hilfe"},
		{"q / Ctrl-c", "beenden"},
	}},
	{"FORMAT", []helpEntry{
		{"12,99", "Betrag"},
		{"TT.MM.JJJJ", "Datum"},
		{"MM.JJJJ", "Monat für neuen Preis"},
	}},
}

func helpColumn(w int, sections []helpSection) []string {
	var out []string
	for i, sec := range sections {
		if i > 0 {
			out = append(out, blank(w, cPanel))
		}
		out = append(out, line(w, cPanel, sg("  "+sec.title, stDimB)))
		for _, e := range sec.entries {
			out = append(out, line(w, cPanel, sg("    "+padRight(e.keys, 18), stAccent), sg(e.desc, stFg)))
		}
	}
	return out
}

func (m *Model) viewHelp(w, h int) []string {
	bw := clampInt(w-4, 40, 96)
	inner := bw - 2
	var body []string
	body = append(body, blank(inner, cPanel))
	if inner >= 88 {
		colW := inner / 2
		l := helpColumn(colW, helpLeft)
		r := helpColumn(inner-colW, helpRight)
		for i := 0; i < max(len(l), len(r)); i++ {
			left, right := blank(colW, cPanel), blank(inner-colW, cPanel)
			if i < len(l) {
				left = l[i]
			}
			if i < len(r) {
				right = r[i]
			}
			body = append(body, left+right)
		}
	} else {
		body = append(body, helpColumn(inner, helpLeft)...)
		body = append(body, blank(inner, cPanel))
		body = append(body, helpColumn(inner, helpRight)...)
	}
	body = append(body, blank(inner, cPanel))
	if maxBody := h - 2; len(body) > maxBody && maxBody > 0 {
		body = body[:maxBody]
	}
	return box(bw, []seg{sg("abo-tracker", stAccB), sg(" · hilfe", stDim)}, body)
}

func (m *Model) viewPalette(w, h int) []string {
	bw := clampInt(w*3/5, min(50, w-2), 80)
	inner := bw - 2
	hits := paletteHits(m.query)
	listH := max(1, min(len(paletteEntries), h-4-2-3))

	summary := ""
	if m.query != "" {
		summary = fmt.Sprintf("  %d Treffer", len(hits))
	}
	body := []string{
		line(inner, cPanel, sg(" > ", stAccB), sg(m.query, stFg), sg("█", stAccent), sg(summary, stDim)),
		line(inner, cPanel, sg(strings.Repeat("─", inner), stBorder)),
	}
	if len(hits) == 0 {
		body = append(body, line(inner, cPanel, sg("  keine Treffer", stDim)))
	} else {
		cur := clampInt(m.paletteCursor, 0, len(hits)-1)
		start := 0
		if cur >= listH {
			start = cur + 1 - listH
		}
		end := min(len(hits), start+listH)
		for i := start; i < end; i++ {
			e := hits[i]
			bg, mark, labelSt := cPanel, "   ", stFg
			if i == cur {
				bg, mark, labelSt = cCursor, " ▶ ", stBold
			}
			gap := max(1, inner-3-lipgloss.Width(e.label)-lipgloss.Width(e.keys)-1)
			body = append(body, line(inner, bg,
				sg(mark, stAccB), sg(e.label+strings.Repeat(" ", gap), labelSt), sg(e.keys, stDim)))
		}
	}
	for len(body) < listH+2 {
		body = append(body, blank(inner, cPanel))
	}
	body = append(body, line(inner, cPanel,
		sg("  ↑↓ / Ctrl-N/P", stDimB), sg(" navigieren · ", stDim),
		sg("Enter", stDimB), sg(" ausführen · ", stDim),
		sg("Esc", stDimB), sg(" abbrechen", stDim)))
	return box(bw, []seg{sg("abo-tracker", stAccB), sg(" · befehl", stDim)}, body)
}

// --- Hilfsfunktionen ------------------------------------------------------

func fill(lines []string, h, w int, bg lipgloss.Color) []string {
	if len(lines) > h {
		return lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, blank(w, bg))
	}
	return lines
}

// keepCursorVisible verschiebt das Fenster nur so weit, dass die
// Cursorzeile sichtbar bleibt.
func keepCursorVisible(prev, cursorLine, height, total int) int {
	if height <= 0 || total == 0 {
		return 0
	}
	maxOff := max(0, total-height)
	off := clampInt(prev, 0, maxOff)
	if cursorLine >= 0 {
		if cursorLine < off {
			off = cursorLine
		} else if cursorLine >= off+height {
			off = cursorLine + 1 - height
		}
	}
	return clampInt(off, 0, maxOff)
}
