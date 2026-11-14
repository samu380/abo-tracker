package sub

import (
	"testing"
	"time"
)

func TestPriceOnAndUpcoming(t *testing.T) {
	today := time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)
	s := Subscription{
		AmountCents: 1299,
		Interval:    IntervalMonth,
		NextBilling: time.Date(2026, 12, 1, 0, 0, 0, 0, time.Local),
		Status:      StatusActive,
	}
	s = s.SetPriceFrom(time.Date(2026, 12, 1, 0, 0, 0, 0, time.Local), 1599, today)
	if got := s.PriceOn(today); got != 1299 {
		t.Fatalf("current %d", got)
	}
	if got := s.PriceOn(s.NextBilling); got != 1599 {
		t.Fatalf("billing %d", got)
	}
	next, ok := s.UpcomingPrice(today)
	if !ok || next.AmountCents != 1599 || next.From.Month() != time.December {
		t.Fatalf("upcoming %+v %v", next, ok)
	}
	sum := Summarize([]Subscription{{
		ID: 1, Name: "Netflix", AmountCents: s.AmountCents, Interval: s.Interval,
		NextBilling: s.NextBilling, Category: "Streaming", Status: StatusActive,
		Prices: s.Prices,
	}}, today)
	if sum.MonthlyCents != 1299 || sum.UpcomingCents != 1599 {
		t.Fatalf("summary %+v", sum)
	}
}

func TestRollForwardUsesPriceAtChargeDate(t *testing.T) {
	today := time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)
	s := Subscription{
		ID: 1, Name: "Netflix", AmountCents: 1000, Interval: IntervalMonth,
		NextBilling: time.Date(2026, 9, 15, 0, 0, 0, 0, time.Local),
		Category:    "Streaming", Status: StatusActive, ChargeMode: ChargeAuto,
	}
	s = s.SetPriceFrom(time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local), 1200, today)
	got, changed := s.RollForward(today)
	if !changed {
		t.Fatal("expected change")
	}
	if got.NextBilling.Month() != time.November || got.NextBilling.Day() != 15 {
		t.Fatalf("next %v", got.NextBilling)
	}
	if len(got.Charges) != 2 {
		t.Fatalf("charges %+v", got.Charges)
	}
	if got.Charges[0].AmountCents != 1000 || got.Charges[0].Date.Month() != time.September {
		t.Fatalf("first %+v", got.Charges[0])
	}
	if got.Charges[1].AmountCents != 1200 || got.Charges[1].Date.Month() != time.October {
		t.Fatalf("second %+v", got.Charges[1])
	}
	if got.AmountCents != 1200 {
		t.Fatalf("current %d", got.AmountCents)
	}

	manual := s
	manual.ChargeMode = ChargeManual
	stayed, _ := manual.RollForward(today)
	if !stayed.NextBilling.Equal(s.NextBilling) {
		t.Fatalf("manual moved to %v", stayed.NextBilling)
	}
	if len(stayed.Charges) != 0 {
		t.Fatalf("manual charges %+v", stayed.Charges)
	}
}

func TestFormatPercentChange(t *testing.T) {
	if got := FormatPercentChange(1299, 1599); got != "+23,1 %" {
		t.Fatalf("increase %q", got)
	}
	if got := FormatPercentChange(2000, 1500); got != "-25,0 %" {
		t.Fatalf("decrease %q", got)
	}
	if got := FormatPercentChange(1000, 1000); got != "+0,0 %" {
		t.Fatalf("same %q", got)
	}
	if got := FormatPercentChange(0, 100); got != "" {
		t.Fatalf("zero base %q", got)
	}
}

func TestParseMonth(t *testing.T) {
	got, err := ParseMonth("03.2027")
	if err != nil {
		t.Fatal(err)
	}
	if got.Year() != 2027 || got.Month() != time.March || got.Day() != 1 {
		t.Fatalf("%v", got)
	}
	if _, err := ParseMonth("13.2027"); err == nil {
		t.Fatal("expected invalid month")
	}
}

func TestSetPriceFromReplacesSameMonth(t *testing.T) {
	today := time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)
	s := Subscription{AmountCents: 1000, Status: StatusActive}
	s = s.SetPriceFrom(time.Date(2027, 3, 1, 0, 0, 0, 0, time.Local), 1500, today)
	s = s.SetPriceFrom(time.Date(2027, 3, 20, 0, 0, 0, 0, time.Local), 1700, today)
	if len(s.Prices) != 2 {
		t.Fatalf("prices %+v", s.Prices)
	}
	if s.PriceOn(time.Date(2027, 3, 1, 0, 0, 0, 0, time.Local)) != 1700 {
		t.Fatalf("march price %d", s.PriceOn(time.Date(2027, 3, 1, 0, 0, 0, 0, time.Local)))
	}
}
