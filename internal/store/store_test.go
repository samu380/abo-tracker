package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"abo-tracker/internal/sub"
)

func sample() sub.Subscription {
	return sub.Subscription{
		ID:          1,
		Name:        "Netflix",
		AmountCents: 1299,
		Interval:    sub.IntervalMonth,
		NextBilling: time.Date(2026, 11, 15, 0, 0, 0, 0, time.Local),
		Category:    "Streaming",
		Status:      sub.StatusActive,
		NoticeDays:  14,
		Note:        "Familie",
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abo", "subscriptions.json")
	if err := Save(path, []sub.Subscription{sample()}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Netflix" || got[0].AmountCents != 1299 || got[0].NoticeDays != 14 {
		t.Fatalf("%+v", got)
	}
	if got[0].NextBilling.Day() != 15 || got[0].NextBilling.Month() != time.November {
		t.Fatalf("date %v", got[0].NextBilling)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"next_billing": "2026-11-15"`) {
		t.Fatalf("json:\n%s", raw)
	}
}

func TestLoadMissing(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestLoadRejectsDuplicateAndGarbage(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"no":"array"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil {
		t.Fatal("expected json error")
	}

	dup := filepath.Join(dir, "dup.json")
	body := `[
	  {"id":1,"name":"A","amount_cents":100,"interval":"monthly","next_billing":"2026-11-01","category":"Software","status":"active","notice_days":0},
	  {"id":1,"name":"B","amount_cents":100,"interval":"monthly","next_billing":"2026-11-01","category":"Software","status":"active","notice_days":0}
	]`
	if err := os.WriteFile(dup, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dup); err == nil {
		t.Fatal("expected duplicate id")
	}
}
