package sub

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func DateOnly(t time.Time) time.Time {
	t = t.In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

func DaysBetween(from, to time.Time) int {
	from = from.In(time.Local)
	to = to.In(time.Local)
	a := time.Date(from.Year(), from.Month(), from.Day(), 12, 0, 0, 0, time.UTC)
	b := time.Date(to.Year(), to.Month(), to.Day(), 12, 0, 0, 0, time.UTC)
	return int(b.Sub(a) / (24 * time.Hour))
}

func FormatDate(t time.Time) string {
	return DateOnly(t).Format("02.01.2006")
}

func FormatMonth(t time.Time) string {
	return MonthStart(t).Format("01.2006")
}

func ParseMonth(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, errors.New("Monat bitte als MM.JJJJ, zum Beispiel 03.2027")
	}
	layouts := []string{"01.2006", "1.2006", "2006-01", "01/2006"}
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return MonthStart(t), nil
		}
	}
	return time.Time{}, errors.New("Monat bitte als MM.JJJJ, zum Beispiel 03.2027")
}

func ParseDate(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, errors.New("Datum bitte als TT.MM.JJJJ")
	}
	layouts := []string{"02.01.2006", "2.1.2006", "2006-01-02"}
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return DateOnly(t), nil
		}
	}
	return time.Time{}, errors.New("Datum bitte als TT.MM.JJJJ")
}

func FormatAmount(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return sign + formatThousands(cents/100) + fmt.Sprintf(",%02d", cents%100)
}

func FormatEUR(cents int64) string {
	return FormatAmount(cents) + " €"
}

// FormatPercentChange vergleicht zwei Beträge in Cent und liefert die Änderung
// mit einer Nachkommastelle, zum Beispiel "+23,1 %".
func FormatPercentChange(from, to int64) string {
	if from <= 0 {
		return ""
	}
	tenths := divRound((to-from)*1000, from)
	sign := "+"
	if tenths < 0 {
		sign = "-"
		tenths = -tenths
	}
	return fmt.Sprintf("%s%d,%d %%", sign, tenths/10, tenths%10)
}

func formatThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	lead := len(s) % 3
	if lead == 0 {
		lead = 3
	}
	var b strings.Builder
	b.WriteString(s[:lead])
	for i := lead; i < len(s); i += 3 {
		b.WriteByte('.')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func ParseAmount(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimSpace(strings.TrimSuffix(s, "€"))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\u00a0", "")
	if s == "" {
		return 0, errors.New("Bitte einen Betrag angeben")
	}
	if strings.HasPrefix(s, "-") {
		return 0, errors.New("Betrag muss größer als 0 sein")
	}
	normalized, err := normalizeAmount(s)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(normalized, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("Betrag nicht erkannt")
	}
	euros, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, errors.New("Betrag nicht erkannt")
	}
	var cents int64
	if len(parts) == 2 {
		frac := parts[1]
		if frac == "" || len(frac) > 2 {
			return 0, errors.New("höchstens zwei Nachkommastellen")
		}
		if len(frac) == 1 {
			frac += "0"
		}
		cents, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, errors.New("Betrag nicht erkannt")
		}
	}
	total := euros*100 + cents
	if total <= 0 {
		return 0, errors.New("Betrag muss größer als 0 sein")
	}
	if total > MaxAmountCents {
		return 0, errors.New("Betrag ist zu hoch")
	}
	return total, nil
}

func normalizeAmount(s string) (string, error) {
	hasComma := strings.Contains(s, ",")
	hasDot := strings.Contains(s, ".")
	switch {
	case hasComma && hasDot:
		if strings.LastIndex(s, ",") > strings.LastIndex(s, ".") {
			s = strings.ReplaceAll(s, ".", "")
			return strings.ReplaceAll(s, ",", "."), nil
		}
		return strings.ReplaceAll(s, ",", ""), nil
	case hasComma:
		if strings.Count(s, ",") != 1 {
			return "", errors.New("Betrag nicht erkannt")
		}
		return strings.ReplaceAll(s, ",", "."), nil
	case hasDot:
		if strings.Count(s, ".") > 1 {
			if !groupedThousands(s, ".") {
				return "", errors.New("Betrag nicht erkannt")
			}
			return strings.ReplaceAll(s, ".", ""), nil
		}
		parts := strings.Split(s, ".")
		if len(parts) != 2 || parts[0] == "" {
			return "", errors.New("Betrag nicht erkannt")
		}
		if len(parts[1]) == 3 && digitsOnly(parts[0]) && digitsOnly(parts[1]) {
			return parts[0] + parts[1], nil
		}
		return s, nil
	default:
		if !digitsOnly(s) {
			return "", errors.New("Betrag nicht erkannt")
		}
		return s, nil
	}
}

func groupedThousands(s, sep string) bool {
	parts := strings.Split(s, sep)
	if len(parts) < 2 || parts[0] == "" || !digitsOnly(parts[0]) {
		return false
	}
	for _, p := range parts[1:] {
		if len(p) != 3 || !digitsOnly(p) {
			return false
		}
	}
	return true
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func ParseNoticeDays(raw string) (int, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 3650 {
		return 0, errors.New("Frist als ganze Tage von 0 bis 3650")
	}
	return n, nil
}

func ParseInterval(s string) (Interval, error) {
	in := Interval(s)
	if !in.Valid() {
		return "", fmt.Errorf("Intervall %q ist ungültig", s)
	}
	return in, nil
}

func ParseStatus(s string) (Status, error) {
	st := Status(s)
	if !st.Valid() {
		return "", fmt.Errorf("Status %q ist ungültig", s)
	}
	return st, nil
}
