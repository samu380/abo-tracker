package sub

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

type ChargeMode string

const (
	ChargeAuto   ChargeMode = "auto"
	ChargeManual ChargeMode = "manual"
)

func (m ChargeMode) OrAuto() ChargeMode {
	if m == ChargeManual {
		return ChargeManual
	}
	return ChargeAuto
}

func (m ChargeMode) Label() string {
	if m == ChargeManual {
		return "Manuell"
	}
	return "Automatisch"
}

type Price struct {
	From        time.Time
	AmountCents int64
}

type Charge struct {
	Date        time.Time
	AmountCents int64
}

func (s Subscription) PriceOn(day time.Time) int64 {
	day = DateOnly(day)
	amount := s.AmountCents
	for _, p := range s.sortedPrices() {
		from := DateOnly(p.From)
		if from.IsZero() || !from.After(day) {
			amount = p.AmountCents
		}
	}
	return amount
}

func (s Subscription) UpcomingPrice(today time.Time) (Price, bool) {
	today = DateOnly(today)
	var found Price
	ok := false
	for _, p := range s.sortedPrices() {
		from := DateOnly(p.From)
		if from.IsZero() || !from.After(today) {
			continue
		}
		if !ok || from.Before(found.From) {
			found = p
			found.From = from
			ok = true
		}
	}
	return found, ok
}

func (s Subscription) SetCurrentPrice(amount int64, today time.Time) Subscription {
	s = s.clone()
	s.ensurePrices()
	today = DateOnly(today)
	idx := 0
	for i, p := range s.Prices {
		from := DateOnly(p.From)
		if from.IsZero() || !from.After(today) {
			idx = i
		}
	}
	s.Prices[idx].AmountCents = amount
	s.AmountCents = s.PriceOn(today)
	return s
}

func (s Subscription) SetPriceFrom(month time.Time, amount int64, today time.Time) Subscription {
	s = s.clone()
	s.ensurePrices()
	month = MonthStart(month)
	replaced := false
	for i := range s.Prices {
		if sameMonth(s.Prices[i].From, month) {
			s.Prices[i].AmountCents = amount
			s.Prices[i].From = month
			replaced = true
			break
		}
	}
	if !replaced {
		s.Prices = append(s.Prices, Price{From: month, AmountCents: amount})
	}
	s.sortPrices()
	s.AmountCents = s.PriceOn(today)
	return s
}

// RollForward schiebt automatische Abos über vergangene Termine und schreibt
// dafür je eine Abbuchung in den Verlauf. Manuelle Abos bleiben stehen.
func (s Subscription) RollForward(today time.Time) (Subscription, bool) {
	s = s.clone()
	s.ChargeMode = s.ChargeMode.OrAuto()
	hadPrices := len(s.Prices) > 0
	s.ensurePrices()
	beforeDate := DateOnly(s.NextBilling)
	beforeAmt := s.AmountCents
	beforeCharges := len(s.Charges)

	if s.Status == StatusActive && s.ChargeMode == ChargeAuto {
		for i := 0; i < 520 && DaysBetween(today, s.NextBilling) < 0; i++ {
			s.Charges = append(s.Charges, Charge{
				Date:        DateOnly(s.NextBilling),
				AmountCents: s.PriceOn(s.NextBilling),
			})
			s.NextBilling = DateOnly(Advance(s.NextBilling, s.Interval))
		}
	}
	s.AmountCents = s.PriceOn(today)
	changed := !DateOnly(s.NextBilling).Equal(beforeDate) ||
		s.AmountCents != beforeAmt ||
		len(s.Charges) != beforeCharges ||
		!hadPrices
	return s, changed
}

func (s Subscription) RecordChargeAndAdvance() Subscription {
	s = s.clone()
	s.ensurePrices()
	s.Charges = append(s.Charges, Charge{
		Date:        DateOnly(s.NextBilling),
		AmountCents: s.PriceOn(s.NextBilling),
	})
	s.NextBilling = DateOnly(Advance(s.NextBilling, s.Interval))
	s.AmountCents = s.PriceOn(time.Now())
	return s
}

func (s Subscription) ChargesNewestFirst() []Charge {
	out := append([]Charge(nil), s.Charges...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Date.After(out[j].Date)
	})
	return out
}

func (s Subscription) clone() Subscription {
	s.Prices = append([]Price(nil), s.Prices...)
	s.Charges = append([]Charge(nil), s.Charges...)
	return s
}

func (s *Subscription) ensurePrices() {
	if len(s.Prices) == 0 && s.AmountCents > 0 {
		s.Prices = []Price{{AmountCents: s.AmountCents}}
	}
	s.sortPrices()
}

func (s Subscription) sortedPrices() []Price {
	out := append([]Price(nil), s.Prices...)
	sort.SliceStable(out, func(i, j int) bool {
		return priceBefore(out[i].From, out[j].From)
	})
	return out
}

func (s *Subscription) sortPrices() {
	sort.SliceStable(s.Prices, func(i, j int) bool {
		return priceBefore(s.Prices[i].From, s.Prices[j].From)
	})
}

func priceBefore(a, b time.Time) bool {
	a = DateOnly(a)
	b = DateOnly(b)
	if a.IsZero() {
		return !b.IsZero()
	}
	if b.IsZero() {
		return false
	}
	return a.Before(b)
}

func sameMonth(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	a = DateOnly(a)
	b = DateOnly(b)
	return a.Year() == b.Year() && a.Month() == b.Month()
}

func MonthStart(t time.Time) time.Time {
	t = t.In(time.Local)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
}

func (s Subscription) validatePrices() error {
	seen := map[string]bool{}
	baseline := 0
	for _, p := range s.Prices {
		if p.AmountCents <= 0 {
			return errors.New("Betrag muss größer als 0 sein")
		}
		if p.AmountCents > MaxAmountCents {
			return errors.New("Betrag ist zu hoch")
		}
		if p.From.IsZero() {
			baseline++
			if baseline > 1 {
				return errors.New("nur ein Startpreis ist erlaubt")
			}
			continue
		}
		key := fmt.Sprintf("%04d-%02d", p.From.Year(), p.From.Month())
		if seen[key] {
			return fmt.Errorf("Preis für %s ist doppelt", FormatMonth(p.From))
		}
		seen[key] = true
	}
	for _, c := range s.Charges {
		if c.Date.IsZero() {
			return errors.New("Abbuchung ohne Datum")
		}
		if c.AmountCents <= 0 || c.AmountCents > MaxAmountCents {
			return errors.New("Abbuchung mit ungültigem Betrag")
		}
	}
	return nil
}
