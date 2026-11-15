package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	cBg        = lipgloss.Color("#1a1d23")
	cPanel     = lipgloss.Color("#1f232b")
	cBorder    = lipgloss.Color("#3a414f")
	cFg        = lipgloss.Color("#c8ccd4")
	cDim       = lipgloss.Color("#6b7280")
	cAccent    = lipgloss.Color("#8aa9c9")
	cCursor    = lipgloss.Color("#3a4150")
	cSelected  = lipgloss.Color("#2f3947")
	cStatusBar = lipgloss.Color("#252a33")
	cStatusFg  = lipgloss.Color("#a8b0bc")
	cModeFg    = lipgloss.Color("#1a1d23")
	cModeBg    = lipgloss.Color("#8aa9c9")
	cRed       = lipgloss.Color("#e07a7a")
	cAmber     = lipgloss.Color("#d4b06a")
	cGreen     = lipgloss.Color("#7aa67a")
	cTeal      = lipgloss.Color("#7fb3a8")
	cCopper    = lipgloss.Color("#c89a6e")
	cDone      = lipgloss.Color("#5a6270")
)

var (
	stFg     = lipgloss.NewStyle().Foreground(cFg)
	stBold   = lipgloss.NewStyle().Foreground(cFg).Bold(true)
	stDim    = lipgloss.NewStyle().Foreground(cDim)
	stDimB   = lipgloss.NewStyle().Foreground(cDim).Bold(true)
	stAccent = lipgloss.NewStyle().Foreground(cAccent)
	stAccB   = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	stBorder = lipgloss.NewStyle().Foreground(cBorder)
	stRed    = lipgloss.NewStyle().Foreground(cRed)
	stAmber  = lipgloss.NewStyle().Foreground(cAmber)
	stGreen  = lipgloss.NewStyle().Foreground(cGreen)
	stDone   = lipgloss.NewStyle().Foreground(cDone)
)

type seg struct {
	text string
	st   lipgloss.Style
	raw  bool
}

func sg(text string, st lipgloss.Style) seg { return seg{text: text, st: st} }

func rawSeg(text string) seg { return seg{text: text, raw: true} }

// line rendert die Segmente auf bg und füllt oder kürzt auf genau w Zellen.
// Jedes Segment bekommt bg, damit ein Reset am Segmentende den Hintergrund
// nicht unterbricht.
func line(w int, bg lipgloss.Color, segs ...seg) string {
	if w <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for _, s := range segs {
		room := w - used
		if room <= 0 {
			break
		}
		if s.raw {
			sw := lipgloss.Width(s.text)
			if sw > room {
				b.WriteString(ansi.Truncate(s.text, room, ""))
				used = w
				break
			}
			b.WriteString(s.text)
			used += sw
			continue
		}
		t := s.text
		if lipgloss.Width(t) > room {
			t = ansi.Truncate(t, room, "")
		}
		st := s.st
		if _, unset := st.GetBackground().(lipgloss.NoColor); unset {
			st = st.Background(bg)
		}
		b.WriteString(st.Render(t))
		used += lipgloss.Width(t)
	}
	if used < w {
		b.WriteString(lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", w-used)))
	}
	return b.String()
}

func blank(w int, bg lipgloss.Color) string { return line(w, bg) }

// box rahmt body mit einer Titelzeile wie "┌ abo · hilfe ───┐".
// body-Zeilen werden auf w-2 Zellen gebracht.
func box(w int, title []seg, body []string) []string {
	if w < 4 {
		w = 4
	}
	inner := w - 2
	titleW := 0
	for _, t := range title {
		titleW += lipgloss.Width(t.text)
	}
	rest := inner - titleW - 3
	if rest < 0 {
		rest = 0
	}
	top := append([]seg{sg("┌─ ", stBorder)}, title...)
	top = append(top, sg(" "+strings.Repeat("─", rest), stBorder), sg("┐", stBorder))
	out := []string{line(w, cPanel, top...)}
	for _, b := range body {
		out = append(out, line(w, cPanel, sg("│", stBorder), rawSeg(fit(b, inner)), sg("│", stBorder)))
	}
	out = append(out, line(w, cPanel, sg("└"+strings.Repeat("─", inner)+"┘", stBorder)))
	return out
}

// fit bringt einen bereits gerenderten String auf genau w Zellen.
func fit(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw > w {
		return ansi.Truncate(s, w, "")
	}
	if sw < w {
		return s + lipgloss.NewStyle().Background(cPanel).Render(strings.Repeat(" ", w-sw))
	}
	return s
}

// overlay legt das Fenster zentriert über die Zeilen.
func overlay(rows []string, win []string, totalW int) []string {
	if len(win) == 0 || len(rows) == 0 {
		return rows
	}
	winW := lipgloss.Width(win[0])
	x := (totalW - winW) / 2
	if x < 0 {
		x = 0
	}
	y := (len(rows) - len(win)) / 2
	if y < 0 {
		y = 0
	}
	out := append([]string(nil), rows...)
	for j, wl := range win {
		r := y + j
		if r >= len(out) {
			break
		}
		left := ansi.Truncate(out[r], x, "")
		right := ansi.TruncateLeft(out[r], x+winW, "")
		out[r] = left + wl + right
	}
	return out
}

func padRight(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw >= w {
		return ansi.Truncate(s, w, "")
	}
	return s + strings.Repeat(" ", w-sw)
}

func padLeft(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw >= w {
		return ansi.Truncate(s, w, "")
	}
	return strings.Repeat(" ", w-sw) + s
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	return ansi.Truncate(s, n, "…")
}

func shortPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	prefix := home + string(os.PathSeparator)
	if strings.HasPrefix(p, prefix) {
		return "~" + p[len(home):]
	}
	return p
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
