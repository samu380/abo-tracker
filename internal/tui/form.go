package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"

	"abo-tracker/internal/sub"
)

const (
	fieldName = iota
	fieldAmount
	fieldInterval
	fieldDate
	fieldCategory
	fieldNotice
	fieldCharge
	fieldNote
	fieldSave
	fieldCount
)

const (
	priceFieldAmount = iota
	priceFieldMonth
	priceFieldSave
	priceFieldCount
)

type fieldKind int

const (
	kindText fieldKind = iota
	kindSelect
	kindAction
)

type formField struct {
	label   string
	kind    fieldKind
	input   textinput.Model
	options []string
	option  int
}

type formKind int

const (
	formSubscription formKind = iota
	formPrice
)

type formState struct {
	kind   formKind
	id     int
	title  string
	index  int
	fields []formField
	err    string
}

type fieldError struct {
	field int
	text  string
}

func (e fieldError) Error() string { return e.text }

func buildForm(existing *sub.Subscription) formState {
	f := formState{
		title:  "Neues Abo",
		fields: make([]formField, fieldCount),
	}
	f.fields[fieldName] = textField("Name", "z. B. Netflix", 80)
	f.fields[fieldAmount] = textField("Betrag (€)", "12,99", 16)
	f.fields[fieldInterval] = selectField("Intervall", intervalLabels(), 0)
	f.fields[fieldDate] = textField("Nächste Abbuchung", "TT.MM.JJJJ", 10)
	f.fields[fieldCategory] = selectField("Kategorie", categoryOptions(""), 0)
	f.fields[fieldNotice] = textField("Frist (Tage)", "0", 4)
	f.fields[fieldCharge] = selectField("Abbuchung", []string{"Automatisch", "Manuell"}, 0)
	f.fields[fieldNote] = textField("Notiz", "optional", 200)
	f.fields[fieldSave] = formField{label: "Speichern", kind: kindAction}
	f.fields[fieldNotice].input.SetValue("0")

	if existing == nil {
		return f
	}
	f.id = existing.ID
	f.title = "Abo bearbeiten"
	f.fields[fieldName].input.SetValue(existing.Name)
	f.fields[fieldAmount].input.SetValue(sub.FormatAmount(existing.AmountCents))
	f.fields[fieldInterval].option = intervalIndex(existing.Interval)
	f.fields[fieldDate].input.SetValue(sub.FormatDate(existing.NextBilling))
	opts := categoryOptions(existing.Category)
	f.fields[fieldCategory].options = opts
	f.fields[fieldCategory].option = indexOf(opts, existing.Category)
	f.fields[fieldNotice].input.SetValue(strconv.Itoa(existing.NoticeDays))
	if existing.ChargeMode == sub.ChargeManual {
		f.fields[fieldCharge].option = 1
	}
	f.fields[fieldNote].input.SetValue(existing.Note)
	return f
}

func buildPriceForm(existing sub.Subscription) formState {
	f := formState{
		kind:   formPrice,
		id:     existing.ID,
		title:  "Preis ab Monat · " + existing.Name,
		fields: make([]formField, priceFieldCount),
	}
	f.fields[priceFieldAmount] = textField("Neuer Betrag (€)", "15,99", 16)
	f.fields[priceFieldMonth] = textField("Ab Monat", "03.2027", 7)
	f.fields[priceFieldSave] = formField{label: "Speichern", kind: kindAction}
	return f
}

func (f formState) toSubscription(existing []sub.Subscription) (sub.Subscription, error) {
	name := strings.TrimSpace(f.fields[fieldName].input.Value())
	if name == "" {
		return sub.Subscription{}, fieldError{field: fieldName, text: "Bitte einen Namen angeben"}
	}
	amount, err := sub.ParseAmount(f.fields[fieldAmount].input.Value())
	if err != nil {
		return sub.Subscription{}, fieldError{field: fieldAmount, text: err.Error()}
	}
	intervalField := f.fields[fieldInterval]
	if intervalField.option < 0 || intervalField.option >= len(sub.Intervals) {
		return sub.Subscription{}, fieldError{field: fieldInterval, text: "Intervall ist ungültig"}
	}
	next, err := sub.ParseDate(f.fields[fieldDate].input.Value())
	if err != nil {
		return sub.Subscription{}, fieldError{field: fieldDate, text: err.Error()}
	}
	catField := f.fields[fieldCategory]
	if catField.option < 0 || catField.option >= len(catField.options) {
		return sub.Subscription{}, fieldError{field: fieldCategory, text: "Bitte eine Kategorie wählen"}
	}
	notice, err := sub.ParseNoticeDays(f.fields[fieldNotice].input.Value())
	if err != nil {
		return sub.Subscription{}, fieldError{field: fieldNotice, text: err.Error()}
	}
	note := strings.TrimSpace(f.fields[fieldNote].input.Value())
	mode := sub.ChargeAuto
	if f.fields[fieldCharge].option == 1 {
		mode = sub.ChargeManual
	}

	s := sub.Subscription{
		Name:        name,
		AmountCents: amount,
		Interval:    sub.Intervals[intervalField.option],
		NextBilling: next,
		Category:    catField.options[catField.option],
		NoticeDays:  notice,
		Note:        note,
		Status:      sub.StatusActive,
		ChargeMode:  mode,
	}
	if f.id != 0 {
		prev, ok := sub.Find(existing, f.id)
		if !ok {
			return sub.Subscription{}, fieldError{field: fieldName, text: "Abo wurde nicht gefunden"}
		}
		s.ID = prev.ID
		s.Status = prev.Status
		s.Prices = append([]sub.Price(nil), prev.Prices...)
		s.Charges = append([]sub.Charge(nil), prev.Charges...)
	} else {
		s.ID = sub.NextID(existing)
	}
	s = s.SetCurrentPrice(amount, time.Now())
	s, _ = s.RollForward(time.Now())
	if err := s.Validate(); err != nil {
		return sub.Subscription{}, fieldError{field: fieldName, text: err.Error()}
	}
	return s, nil
}

func textField(label, placeholder string, limit int) formField {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = placeholder
	ti.CharLimit = limit
	ti.Width = 32
	ti.PlaceholderStyle = mutedStyle
	return formField{label: label, kind: kindText, input: ti}
}

func selectField(label string, options []string, selected int) formField {
	if selected < 0 || selected >= len(options) {
		selected = 0
	}
	return formField{
		label:   label,
		kind:    kindSelect,
		options: options,
		option:  selected,
	}
}

func intervalLabels() []string {
	labels := make([]string, len(sub.Intervals))
	for i, in := range sub.Intervals {
		labels[i] = in.Label()
	}
	return labels
}

func categoryOptions(current string) []string {
	opts := append([]string{}, sub.Categories...)
	if current == "" {
		return opts
	}
	for _, opt := range opts {
		if opt == current {
			return opts
		}
	}
	return append(opts, current)
}

func intervalIndex(v sub.Interval) int {
	for i, in := range sub.Intervals {
		if in == v {
			return i
		}
	}
	return 0
}

func indexOf(options []string, want string) int {
	for i, o := range options {
		if o == want {
			return i
		}
	}
	return 0
}
