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
		return "Fenster zu klein.\nMindestens 56×20.\n\nctrl+c beendet.\n"
	}
	var body string
	switch m.screen {
	case screenForm:
		body = m.viewForm()
	case screenConfirm:
		body = m.viewConfirm()
	case screenHelp:
		body = m.viewHelp()
	case screenHistory:
		body = m.viewHistory()
	default:
		body = m.viewList()
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}

func (m *Model) viewList() string {
	today := time.Now()
	w := m.contentWidth()
	vis := m.visibleSubs()
	sum := sub.Summarize(m.subs, today)

	var b strings.Builder
	right := fmt.Sprintf("%d aktiv", sum.Active)
	if sum.Cancelled > 0 {
		right += fmt.Sprintf(" · %d gekündigt", sum.Cancelled)
	}
	b.WriteString(spread(titleStyle.Render("Abos"), mutedStyle.Render(right), w))
	b.WriteString("\n\n")
	b.WriteString(statLine("Monatlich aktiv", sub.FormatEUR(sum.MonthlyCents), okStyle, w))
	b.WriteString("\n")
	b.WriteString(statLine("Nächste 30 Tage", sub.FormatEUR(sum.UpcomingCents), lipgloss.NewStyle(), w))
	b.WriteString("\n")
	overdueStyle := mutedStyle
	if sum.OverdueCents > 0 {
		overdueStyle = warnStyle
	}
	b.WriteString(statLine("Überfällig", sub.FormatEUR(sum.OverdueCents), overdueStyle, w))
	b.WriteString("\n\n")

	switch {
	case len(m.subs) == 0:
		b.WriteString("Noch keine Abos.\n\n")
		b.WriteString(mutedStyle.Render("Mit a legst du das erste an."))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Beispiel: Netflix, 12,99 € monatlich."))
	case len(vis) == 0:
		b.WriteString("Keine aktiven Abos.\n\n")
		b.WriteString(mutedStyle.Render("f zeigt auch gekündigte."))
	default:
		end := m.offset + m.pageSize()
		if end > len(vis) {
			end = len(vis)
		}
		if m.offset < 0 {
			m.offset = 0
		}
		for i := m.offset; i < end; i++ {
			b.WriteString(m.viewItem(vis[i], i == m.cursor, w, today))
			b.WriteString("\n")
		}
	}

	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(m.renderStatus())
	}
	b.WriteString("\n\n")
	current, ok := selected(vis, m.cursor)
	var currentPtr *sub.Subscription
	if ok {
		currentPtr = &current
	}
	b.WriteString(m.helpList(currentPtr))
	b.WriteString("\n")

	info := ""
	if len(vis) > m.pageSize() {
		end := m.offset + m.pageSize()
		if end > len(vis) {
			end = len(vis)
		}
		info = fmt.Sprintf("%d–%d von %d", m.offset+1, end, len(vis))
	}
	b.WriteString(spread(mutedStyle.Render(info), mutedStyle.Render(shortPath(m.path)), w))
	return b.String()
}

func (m *Model) viewItem(s sub.Subscription, isSelected bool, width int, today time.Time) string {
	price := sub.FormatEUR(s.PriceOn(today)) + " / " + s.Interval.Per()
	priceW := lipgloss.Width(price)
	marker := "  "
	if isSelected {
		marker = "> "
	}
	budget := width - priceW - 1
	if budget < 3 {
		budget = 3
	}
	left := marker + truncate(s.Name, budget-2)
	gap := width - lipgloss.Width(left) - priceW
	if gap < 1 {
		gap = 1
	}

	att := s.AttentionAt(today)
	style := nameStyle
	switch {
	case isSelected:
		style = selectedName
	case s.Status == sub.StatusCancelled:
		style = mutedStyle
	case att == sub.AttentionOverdue || att == sub.AttentionNoticeMissed:
		style = warnStyle
	}
	priceStyle := lipgloss.NewStyle()
	if isSelected {
		priceStyle = priceStyle.Foreground(colorAccent)
	}
	line1 := style.Render(left) + strings.Repeat(" ", gap) + priceStyle.Render(price)

	meta, urgent := metaLine(s, today)
	meta = truncate(meta, width-2)
	metaStyle := mutedStyle
	if urgent {
		metaStyle = warnStyle
	}
	line2 := metaStyle.Render("  " + meta)
	return line1 + "\n" + line2
}

func metaLine(s sub.Subscription, today time.Time) (string, bool) {
	parts := []string{s.Category}
	if s.Interval != sub.IntervalMonth {
		parts = append(parts, "≈ "+sub.FormatEUR(s.MonthlyCents())+"/Monat")
	}
	days := sub.DaysBetween(today, s.NextBilling)
	if s.Status == sub.StatusActive && days < 0 {
		n := -days
		if n == 1 {
			parts = append(parts, "seit gestern überfällig")
		} else {
			parts = append(parts, fmt.Sprintf("seit %d Tagen überfällig", n))
		}
	} else {
		label := sub.FormatDate(s.NextBilling)
		if s.Status == sub.StatusActive && days == 0 {
			label += " · heute"
		} else if s.Status == sub.StatusActive && days == 1 {
			label += " · morgen"
		}
		parts = append(parts, label)
	}
	if s.NoticeDays > 0 && s.Status == sub.StatusActive {
		ndays := sub.DaysBetween(today, sub.NoticeDate(s))
		switch {
		case ndays < 0:
			parts = append(parts, "Frist abgelaufen")
		case ndays == 0:
			parts = append(parts, "Frist heute")
		case ndays == 1:
			parts = append(parts, "Frist morgen")
		case ndays <= 14:
			parts = append(parts, fmt.Sprintf("Frist in %d Tagen", ndays))
		default:
			parts = append(parts, "Frist "+sub.FormatDate(sub.NoticeDate(s)))
		}
	}
	if next, ok := s.UpcomingPrice(today); ok {
		parts = append(parts, "ab "+sub.FormatMonth(next.From)+" "+sub.FormatEUR(next.AmountCents))
	}
	if s.ChargeMode == sub.ChargeManual && s.Status == sub.StatusActive {
		parts = append(parts, "manuell")
	}
	if s.Note != "" {
		parts = append(parts, s.Note)
	}
	if s.Status == sub.StatusCancelled {
		parts = append(parts, "gekündigt")
	}
	att := s.AttentionAt(today)
	urgent := att == sub.AttentionOverdue || att == sub.AttentionNotice || att == sub.AttentionNoticeMissed || att == sub.AttentionSoon
	return strings.Join(parts, " · "), urgent
}

func statLine(label, value string, valueStyle lipgloss.Style, width int) string {
	return spread(mutedStyle.Render(label), valueStyle.Render(value), width)
}

func (m *Model) renderStatus() string {
	style := mutedStyle
	switch m.statusKind {
	case statusOK:
		style = okStyle
	case statusErr:
		style = dangerStyle
	}
	return style.Render(m.status)
}

func (m *Model) viewForm() string {
	w := m.contentWidth()
	const labelW = 22
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.form.title))
	b.WriteString("\n\n")
	for i, f := range m.form.fields {
		if f.kind == kindAction {
			b.WriteString("\n")
		}
		label := f.label
		if i == m.form.index {
			label = activeLabel.Render(label)
		} else {
			label = labelStyle.Render(label)
		}
		value := m.fieldValue(i, f)
		b.WriteString(formRow(label, labelW, value, w))
		b.WriteString("\n")
	}
	if m.form.err != "" {
		b.WriteString("\n")
		b.WriteString(dangerStyle.Render(m.form.err))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑↓ Feld   ←→ Auswahl   enter weiter   esc zurück"))
	return b.String()
}

func (m *Model) fieldValue(i int, f formField) string {
	switch f.kind {
	case kindSelect:
		if f.option < 0 || f.option >= len(f.options) {
			return ""
		}
		text := "‹ " + f.options[f.option] + " ›"
		if i == m.form.index {
			return activeLabel.Render(text)
		}
		return text
	case kindAction:
		if i == m.form.index {
			return okStyle.Render("enter")
		}
		return mutedStyle.Render("enter")
	default:
		return f.input.View()
	}
}

func formRow(label string, labelW int, value string, width int) string {
	pad := labelW - lipgloss.Width(label)
	if pad < 1 {
		pad = 1
	}
	row := label + strings.Repeat(" ", pad) + value
	if lipgloss.Width(row) > width && width > 0 {
		return truncate(row, width)
	}
	return row
}

func (m *Model) viewHistory() string {
	w := m.contentWidth()
	s, ok := sub.Find(m.subs, m.historyID)
	var b strings.Builder
	if !ok {
		b.WriteString(titleStyle.Render("Verlauf"))
		b.WriteString("\n\n")
		b.WriteString("Abo wurde nicht gefunden.")
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("esc zurück"))
		return b.String()
	}
	b.WriteString(spread(titleStyle.Render("Verlauf"), mutedStyle.Render(s.Name), w))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(s.ChargeMode.OrAuto().Label()))
	b.WriteString("\n\n")
	lines := historyBodyLines(s)
	m.clampHistory()
	end := m.historyOffset + m.historyPage()
	if end > len(lines) {
		end = len(lines)
	}
	if m.historyOffset < 0 {
		m.historyOffset = 0
	}
	for _, line := range lines[m.historyOffset:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑↓ blättern    esc zurück"))
	return b.String()
}

func historyBodyLines(s sub.Subscription) []string {
	lines := []string{"Preise"}
	prices := orderedPrices(s)
	if len(prices) == 0 {
		lines = append(lines, "  "+sub.FormatEUR(s.AmountCents)+"    seit Beginn")
	} else {
		for i, p := range prices {
			var until string
			if p.From.IsZero() {
				until = "seit Beginn"
			} else {
				until = "ab " + sub.FormatMonth(p.From)
			}
			if i+1 < len(prices) && !prices[i+1].From.IsZero() {
				prev := prices[i+1].From.AddDate(0, -1, 0)
				if p.From.IsZero() {
					until = "bis " + sub.FormatMonth(prev)
				} else {
					until = sub.FormatMonth(p.From) + " – " + sub.FormatMonth(prev)
				}
			}
			amount := sub.FormatEUR(p.AmountCents)
			if i > 0 {
				amount += "   " + sub.FormatPercentChange(prices[i-1].AmountCents, p.AmountCents)
			}
			lines = append(lines, fmt.Sprintf("  %-22s %s", until, amount))
		}
	}
	lines = append(lines, "", "Abbuchungen")
	charges := s.ChargesNewestFirst()
	if len(charges) == 0 {
		lines = append(lines, "  Noch keine Abbuchungen.")
		return lines
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

func (m *Model) viewConfirm() string {
	w := m.contentWidth()
	var b strings.Builder
	b.WriteString(titleStyle.Render("Abo löschen"))
	b.WriteString("\n\n")
	b.WriteString(selectedName.Render(truncate(m.pending.Name, w)))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(sub.FormatEUR(m.pending.AmountCents) + " / " + m.pending.Interval.Per()))
	b.WriteString("\n\n")
	b.WriteString("Der Eintrag wird aus der Datei entfernt.")
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("j löschen    esc abbrechen"))
	return b.String()
}

func (m *Model) viewHelp() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Hilfe"))
	b.WriteString("\n\n")
	b.WriteString("Liste\n")
	b.WriteString(mutedStyle.Render("a"))
	b.WriteString("  neues Abo\n")
	b.WriteString(mutedStyle.Render("e"))
	b.WriteString("  bearbeiten\n")
	b.WriteString(mutedStyle.Render("c"))
	b.WriteString("  kündigen oder wieder aktivieren\n")
	b.WriteString(mutedStyle.Render("n"))
	b.WriteString("  manuelle Abbuchung verbucht, Termin weiter\n")
	b.WriteString(mutedStyle.Render("p"))
	b.WriteString("  neuer Preis ab einem Monat\n")
	b.WriteString(mutedStyle.Render("h"))
	b.WriteString("  Preis- und Abbuchungsverlauf\n")
	b.WriteString(mutedStyle.Render("d"))
	b.WriteString("  löschen, danach j bestätigen\n")
	b.WriteString(mutedStyle.Render("f"))
	b.WriteString("  nur aktive Abos oder alle\n")
	b.WriteString(mutedStyle.Render("q"))
	b.WriteString("  beenden\n\n")
	b.WriteString("Formular\n")
	b.WriteString(mutedStyle.Render("↑↓"))
	b.WriteString("     Feld wechseln\n")
	b.WriteString(mutedStyle.Render("←→"))
	b.WriteString("     Intervall, Kategorie und Abbuchung\n")
	b.WriteString(mutedStyle.Render("enter"))
	b.WriteString("  nächstes Feld, auf Speichern sichern\n")
	b.WriteString(mutedStyle.Render("esc"))
	b.WriteString("    zurück ohne Speichern\n\n")
	b.WriteString(mutedStyle.Render("Betrag als 12,99. Datum als TT.MM.JJJJ. Monat als 03.2027."))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Automatische Abos rücken den Termin selbst weiter."))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Jahres- und Quartalsbeträge zählen anteilig im Monat."))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("esc zurück"))
	return b.String()
}
