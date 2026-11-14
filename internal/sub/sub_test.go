package sub

import (
	"testing"
	"time"
)

func TestMonthlyCents(t *testing.T) {
	cases := []struct {
		name     string
		interval Interval
		amount   int64
		want     int64
	}{
		{"month", IntervalMonth, 1299, 1299},
		{"year", IntervalYear, 12000, 1000},
		{"year rounds", IntervalYear, 1199, 100},
		{"quarter", IntervalQuarter, 3000, 1000},
		{"week", IntervalWeek, 500, 2167},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Subscription{AmountCents: tc.amount, Interval: tc.interval}
			if got := s.MonthlyCents(); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"12,99", 1299, false},
		{"12.99", 1299, false},
		{"12,9", 1290, false},
		{"8", 800, false},
		{"1.234,56", 123456, false},
		{"1,234.56", 123456, false},
		{"1.000", 100000, false},
		{"1.000.000,00", MaxAmountCents, false},
		{"0", 0, true},
		{"0,00", 0, true},
		{"-1", 0, true},
		{"", 0, true},
		{"abc", 0, true},
		{"12,999", 0, true},
		{"1000000,01", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseAmount(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFormatAmountRoundTrip(t *testing.T) {
	for _, cents := range []int64{50, 1299, 100000, 123456, MaxAmountCents} {
		got, err := ParseAmount(FormatAmount(cents))
		if err != nil {
			t.Fatal(err)
		}
		if got != cents {
			t.Fatalf("roundtrip %d -> %s -> %d", cents, FormatAmount(cents), got)
		}
	}
}

func TestParseDate(t *testing.T) {
	got, err := ParseDate("15.11.2026")
	if err != nil {
		t.Fatal(err)
	}
	if got.Year() != 2026 || got.Month() != time.November || got.Day() != 15 {
		t.Fatalf("got %v", got)
	}
	iso, err := ParseDate("2026-11-15")
	if err != nil {
		t.Fatal(err)
	}
	if !iso.Equal(got) {
		t.Fatalf("iso %v german %v", iso, got)
	}
	if _, err := ParseDate("29.02.2026"); err == nil {
		t.Fatal("expected invalid leap day")
	}
	if _, err := ParseDate(""); err == nil {
		t.Fatal("expected empty error")
	}
}

func TestAttentionAndSummary(t *testing.T) {
	today := time.Date(2026, 10, 29, 15, 0, 0, 0, time.Local)
	soon := Subscription{
		ID: 1, Name: "Netflix", AmountCents: 1299, Interval: IntervalMonth,
		NextBilling: time.Date(2026, 11, 3, 0, 0, 0, 0, time.Local),
		Category:    "Streaming", Status: StatusActive, NoticeDays: 14,
	}
	if got := soon.AttentionAt(today); got != AttentionNoticeMissed {
		t.Fatalf("attention %d", got)
	}
	overdue := soon
	overdue.NextBilling = time.Date(2026, 10, 20, 0, 0, 0, 0, time.Local)
	overdue.NoticeDays = 0
	if got := overdue.AttentionAt(today); got != AttentionOverdue {
		t.Fatalf("overdue attention %d", got)
	}

	later := soon
	later.ID = 4
	later.Name = "Spotify"
	later.NoticeDays = 0
	later.NextBilling = time.Date(2026, 12, 20, 0, 0, 0, 0, time.Local)
	year := Subscription{
		ID: 2, Name: "Adobe", AmountCents: 12000, Interval: IntervalYear,
		NextBilling: today.AddDate(0, 0, 10),
		Category:    "Software", Status: StatusActive,
	}
	cancelled := Subscription{
		ID: 3, Name: "Gym", AmountCents: 3000, Interval: IntervalMonth,
		NextBilling: today, Category: "Fitness", Status: StatusCancelled,
	}
	sum := Summarize([]Subscription{later, year, cancelled, overdue}, today)
	if sum.Active != 3 || sum.Cancelled != 1 {
		t.Fatalf("counts %+v", sum)
	}
	if sum.MonthlyCents != 1299+1000+1299 {
		t.Fatalf("monthly %d", sum.MonthlyCents)
	}
	if sum.UpcomingCents != 12000 {
		t.Fatalf("upcoming %d", sum.UpcomingCents)
	}
	if sum.OverdueCents != 1299 {
		t.Fatalf("overdue %d", sum.OverdueCents)
	}
}

func TestVisibleSortsActiveFirst(t *testing.T) {
	day := func(d int) time.Time {
		return time.Date(2026, 11, d, 0, 0, 0, 0, time.Local)
	}
	subs := []Subscription{
		{ID: 1, Name: "Später", Status: StatusActive, NextBilling: day(20)},
		{ID: 2, Name: "Gekündigt", Status: StatusCancelled, NextBilling: day(1)},
		{ID: 3, Name: "Bald", Status: StatusActive, NextBilling: day(5)},
	}
	got := Visible(subs, false)
	if got[0].Name != "Bald" || got[1].Name != "Später" || got[2].Name != "Gekündigt" {
		t.Fatalf("%s, %s, %s", got[0].Name, got[1].Name, got[2].Name)
	}
	only := Visible(subs, true)
	if len(only) != 2 {
		t.Fatalf("len %d", len(only))
	}
}

func TestAdvance(t *testing.T) {
	start := time.Date(2026, 11, 15, 0, 0, 0, 0, time.Local)
	month := Advance(start, IntervalMonth)
	if month.Year() != 2026 || month.Month() != time.December || month.Day() != 15 {
		t.Fatalf("month %v", month)
	}
	week := Advance(start, IntervalWeek)
	if week.Month() != time.November || week.Day() != 22 {
		t.Fatalf("week %v", week)
	}
	year := Advance(start, IntervalYear)
	if year.Year() != 2027 || year.Month() != time.November || year.Day() != 15 {
		t.Fatalf("year %v", year)
	}
}
