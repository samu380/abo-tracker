package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#5EEAD4"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"}
	colorWarn   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FCD34D"}
	colorDanger = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"}
	colorOK     = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#6EE7B7"}
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	mutedStyle   = lipgloss.NewStyle().Foreground(colorMuted)
	warnStyle    = lipgloss.NewStyle().Foreground(colorWarn)
	dangerStyle  = lipgloss.NewStyle().Foreground(colorDanger)
	okStyle      = lipgloss.NewStyle().Foreground(colorOK)
	nameStyle    = lipgloss.NewStyle().Bold(true)
	selectedName = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	helpStyle    = lipgloss.NewStyle().Foreground(colorMuted)
	labelStyle   = lipgloss.NewStyle().Foreground(colorMuted)
	activeLabel  = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
)

func spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		if left == "" {
			return right
		}
		return left + " " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

func truncate(s string, n int) string {
	r := []rune(s)
	if n <= 0 {
		return ""
	}
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
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
