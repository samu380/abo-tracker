package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"abo-tracker/internal/sub"
)

type filePrice struct {
	From        string `json:"from,omitempty"`
	AmountCents int64  `json:"amount_cents"`
}

type fileCharge struct {
	Date        string `json:"date"`
	AmountCents int64  `json:"amount_cents"`
}

type fileSub struct {
	ID          int          `json:"id"`
	Name        string       `json:"name"`
	AmountCents int64        `json:"amount_cents"`
	Interval    string       `json:"interval"`
	NextBilling string       `json:"next_billing"`
	Category    string       `json:"category"`
	Status      string       `json:"status"`
	NoticeDays  int          `json:"notice_days"`
	Note        string       `json:"note,omitempty"`
	ChargeMode  string       `json:"charge_mode,omitempty"`
	Prices      []filePrice  `json:"prices,omitempty"`
	Charges     []fileCharge `json:"charges,omitempty"`
}

func Load(path string) ([]sub.Subscription, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var raw []fileSub
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("JSON ungültig: %w", err)
	}
	out := make([]sub.Subscription, 0, len(raw))
	seen := make(map[int]bool, len(raw))
	for i, item := range raw {
		s, err := item.toDomain()
		if err != nil {
			return nil, fmt.Errorf("Eintrag %d: %w", i+1, err)
		}
		if seen[s.ID] {
			return nil, fmt.Errorf("Eintrag %d: doppelte ID %d", i+1, s.ID)
		}
		seen[s.ID] = true
		out = append(out, s)
	}
	return out, nil
}

func Save(path string, subs []sub.Subscription) error {
	if path == "" {
		return errors.New("kein Dateipfad")
	}
	raw := make([]fileSub, 0, len(subs))
	for _, s := range subs {
		if err := s.Validate(); err != nil {
			return err
		}
		raw = append(raw, fromDomain(s))
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (f fileSub) toDomain() (sub.Subscription, error) {
	if f.ID <= 0 {
		return sub.Subscription{}, errors.New("ID fehlt")
	}
	interval, err := sub.ParseInterval(f.Interval)
	if err != nil {
		return sub.Subscription{}, err
	}
	status, err := sub.ParseStatus(f.Status)
	if err != nil {
		return sub.Subscription{}, err
	}
	next, err := sub.ParseDate(f.NextBilling)
	if err != nil {
		return sub.Subscription{}, err
	}
	prices, err := pricesFromFile(f.Prices)
	if err != nil {
		return sub.Subscription{}, err
	}
	charges, err := chargesFromFile(f.Charges)
	if err != nil {
		return sub.Subscription{}, err
	}
	mode := sub.ChargeAuto
	if f.ChargeMode != "" {
		switch sub.ChargeMode(f.ChargeMode) {
		case sub.ChargeAuto, sub.ChargeManual:
			mode = sub.ChargeMode(f.ChargeMode)
		default:
			return sub.Subscription{}, fmt.Errorf("Abbuchung %q ist ungültig", f.ChargeMode)
		}
	}
	s := sub.Subscription{
		ID:          f.ID,
		Name:        f.Name,
		AmountCents: f.AmountCents,
		Interval:    interval,
		NextBilling: next,
		Category:    f.Category,
		Status:      status,
		NoticeDays:  f.NoticeDays,
		Note:        f.Note,
		ChargeMode:  mode,
		Prices:      prices,
		Charges:     charges,
	}
	if err := s.Validate(); err != nil {
		return sub.Subscription{}, err
	}
	return s, nil
}

func pricesFromFile(raw []filePrice) ([]sub.Price, error) {
	out := make([]sub.Price, 0, len(raw))
	for _, p := range raw {
		var from time.Time
		if p.From != "" {
			parsed, err := sub.ParseMonth(p.From)
			if err != nil {
				parsed, err = sub.ParseDate(p.From)
				if err != nil {
					return nil, fmt.Errorf("Preis ab %q ist ungültig", p.From)
				}
				parsed = sub.MonthStart(parsed)
			}
			from = parsed
		}
		out = append(out, sub.Price{From: from, AmountCents: p.AmountCents})
	}
	return out, nil
}

func chargesFromFile(raw []fileCharge) ([]sub.Charge, error) {
	out := make([]sub.Charge, 0, len(raw))
	for _, c := range raw {
		date, err := sub.ParseDate(c.Date)
		if err != nil {
			return nil, fmt.Errorf("Abbuchung %q: %w", c.Date, err)
		}
		out = append(out, sub.Charge{Date: date, AmountCents: c.AmountCents})
	}
	return out, nil
}

func fromDomain(s sub.Subscription) fileSub {
	prices := make([]filePrice, 0, len(s.Prices))
	for _, p := range s.Prices {
		item := filePrice{AmountCents: p.AmountCents}
		if !p.From.IsZero() {
			item.From = p.From.Format("2006-01")
		}
		prices = append(prices, item)
	}
	charges := make([]fileCharge, 0, len(s.Charges))
	for _, c := range s.Charges {
		charges = append(charges, fileCharge{
			Date:        c.Date.Format("2006-01-02"),
			AmountCents: c.AmountCents,
		})
	}
	return fileSub{
		ID:          s.ID,
		Name:        s.Name,
		AmountCents: s.AmountCents,
		Interval:    string(s.Interval),
		NextBilling: s.NextBilling.Format("2006-01-02"),
		Category:    s.Category,
		Status:      string(s.Status),
		NoticeDays:  s.NoticeDays,
		Note:        s.Note,
		ChargeMode:  string(s.ChargeMode.OrAuto()),
		Prices:      prices,
		Charges:     charges,
	}
}
