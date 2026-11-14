package sub

import (
	"errors"
	"strings"
	"time"
)

const MaxAmountCents int64 = 100_000_000 // 1.000.000,00 €

type Interval string

const (
	IntervalMonth   Interval = "monthly"
	IntervalQuarter Interval = "quarterly"
	IntervalYear    Interval = "yearly"
	IntervalWeek    Interval = "weekly"
)

var Intervals = []Interval{
	IntervalMonth,
	IntervalQuarter,
	IntervalYear,
	IntervalWeek,
}

func (i Interval) Valid() bool {
	switch i {
	case IntervalMonth, IntervalQuarter, IntervalYear, IntervalWeek:
		return true
	default:
		return false
	}
}

func (i Interval) Label() string {
	switch i {
	case IntervalQuarter:
		return "Vierteljährlich"
	case IntervalYear:
		return "Jährlich"
	case IntervalWeek:
		return "Wöchentlich"
	default:
		return "Monatlich"
	}
}

func (i Interval) Per() string {
	switch i {
	case IntervalQuarter:
		return "Quartal"
	case IntervalYear:
		return "Jahr"
	case IntervalWeek:
		return "Woche"
	default:
		return "Monat"
	}
}

type Status string

const (
	StatusActive    Status = "active"
	StatusCancelled Status = "cancelled"
)

func (s Status) Valid() bool {
	return s == StatusActive || s == StatusCancelled
}

var Categories = []string{
	"Streaming",
	"Mobilfunk",
	"Internet",
	"Versicherung",
	"Software",
	"Fitness",
	"Mitgliedschaft",
	"Sonstiges",
}

type Subscription struct {
	ID          int
	Name        string
	AmountCents int64
	Interval    Interval
	NextBilling time.Time
	Category    string
	Status      Status
	NoticeDays  int
	Note        string
	ChargeMode  ChargeMode
	Prices      []Price
	Charges     []Charge
}

func (s Subscription) Validate() error {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return errors.New("Bitte einen Namen angeben")
	}
	if len([]rune(name)) > 80 {
		return errors.New("Name ist zu lang")
	}
	if s.AmountCents <= 0 {
		return errors.New("Betrag muss größer als 0 sein")
	}
	if s.AmountCents > MaxAmountCents {
		return errors.New("Betrag ist zu hoch")
	}
	if !s.Interval.Valid() {
		return errors.New("Intervall ist ungültig")
	}
	if s.NextBilling.IsZero() {
		return errors.New("Datum bitte als TT.MM.JJJJ")
	}
	if strings.TrimSpace(s.Category) == "" {
		return errors.New("Bitte eine Kategorie wählen")
	}
	if !s.Status.Valid() {
		return errors.New("Status ist ungültig")
	}
	if s.NoticeDays < 0 || s.NoticeDays > 3650 {
		return errors.New("Frist als ganze Tage von 0 bis 3650")
	}
	if len([]rune(s.Note)) > 200 {
		return errors.New("Notiz ist zu lang")
	}
	if s.ID <= 0 {
		return errors.New("ID fehlt")
	}
	if s.ChargeMode != "" && s.ChargeMode != ChargeAuto && s.ChargeMode != ChargeManual {
		return errors.New("Abbuchung ist ungültig")
	}
	if err := s.validatePrices(); err != nil {
		return err
	}
	return nil
}

func (s Subscription) MonthlyCents() int64 {
	return MonthlyEquivalent(s.AmountCents, s.Interval)
}

func MonthlyEquivalent(cents int64, interval Interval) int64 {
	switch interval {
	case IntervalYear:
		return divRound(cents, 12)
	case IntervalQuarter:
		return divRound(cents, 3)
	case IntervalWeek:
		return divRound(cents*52, 12)
	default:
		return cents
	}
}

func divRound(n, d int64) int64 {
	if d == 0 {
		return 0
	}
	if n >= 0 {
		return (n + d/2) / d
	}
	return (n - d/2) / d
}

func Advance(t time.Time, interval Interval) time.Time {
	switch interval {
	case IntervalYear:
		return t.AddDate(1, 0, 0)
	case IntervalQuarter:
		return t.AddDate(0, 3, 0)
	case IntervalWeek:
		return t.AddDate(0, 0, 7)
	default:
		return t.AddDate(0, 1, 0)
	}
}

func NoticeDate(s Subscription) time.Time {
	return DateOnly(s.NextBilling).AddDate(0, 0, -s.NoticeDays)
}

type Attention int

const (
	AttentionNone Attention = iota
	AttentionSoon
	AttentionNotice
	AttentionNoticeMissed
	AttentionOverdue
)

func (s Subscription) AttentionAt(today time.Time) Attention {
	if s.Status != StatusActive {
		return AttentionNone
	}
	days := DaysBetween(today, s.NextBilling)
	if days < 0 {
		return AttentionOverdue
	}
	if s.NoticeDays > 0 {
		ndays := DaysBetween(today, NoticeDate(s))
		if ndays < 0 {
			return AttentionNoticeMissed
		}
		if ndays <= 14 {
			return AttentionNotice
		}
	}
	if days <= 7 {
		return AttentionSoon
	}
	return AttentionNone
}

type Summary struct {
	Active        int
	Cancelled     int
	MonthlyCents  int64
	UpcomingCents int64
	OverdueCents  int64
}

func Summarize(subs []Subscription, today time.Time) Summary {
	var sum Summary
	for _, s := range subs {
		if s.Status != StatusActive {
			if s.Status == StatusCancelled {
				sum.Cancelled++
			}
			continue
		}
		sum.Active++
		sum.MonthlyCents += MonthlyEquivalent(s.PriceOn(today), s.Interval)
		days := DaysBetween(today, s.NextBilling)
		charge := s.PriceOn(s.NextBilling)
		if days < 0 {
			sum.OverdueCents += charge
			continue
		}
		if days <= 30 {
			sum.UpcomingCents += charge
		}
	}
	return sum
}

func Visible(subs []Subscription, activeOnly bool) []Subscription {
	out := make([]Subscription, 0, len(subs))
	for _, s := range subs {
		if activeOnly && s.Status != StatusActive {
			continue
		}
		out = append(out, s)
	}
	sortVisible(out)
	return out
}

func Find(subs []Subscription, id int) (Subscription, bool) {
	for _, s := range subs {
		if s.ID == id {
			return s, true
		}
	}
	return Subscription{}, false
}

func NextID(subs []Subscription) int {
	maxID := 0
	for _, s := range subs {
		if s.ID > maxID {
			maxID = s.ID
		}
	}
	return maxID + 1
}
