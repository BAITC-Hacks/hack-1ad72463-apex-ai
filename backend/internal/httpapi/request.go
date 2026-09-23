package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
)

// decodeRequest rejects duplicate keys, trailing JSON, null required fields and
// fractional/exponent budgets. A map alone would silently accept duplicate keys.
func decodeRequest(data []byte) (domain.Request, []domain.FieldError, error) {
	var r domain.Request
	errs := []domain.FieldError{}
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil {
		return r, errs, err
	}
	if token != json.Delim('{') {
		return r, errs, fmt.Errorf("expected JSON object")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		t, err := d.Token()
		if err != nil {
			return r, errs, err
		}
		key, ok := t.(string)
		if !ok {
			return r, errs, fmt.Errorf("invalid key")
		}
		var raw json.RawMessage
		if err = d.Decode(&raw); err != nil {
			return r, errs, err
		}
		if _, ok := fields[key]; ok {
			errs = append(errs, domain.FieldError{Field: key, Code: "INVALID_TYPE", Message: "Поле не должно повторяться."})
		}
		fields[key] = raw
	}
	if _, err = d.Token(); err != nil {
		return r, errs, err
	}
	var trailing any
	if err = d.Decode(&trailing); err != io.EOF {
		return r, errs, fmt.Errorf("unexpected trailing JSON")
	}
	known := map[string]any{"city": &r.City, "date": &r.Date, "event_format": &r.EventFormat, "category": &r.Category, "budget_kzt": &r.Budget, "duration_hours": &r.Duration, "language": &r.Language}
	for _, key := range []string{"city", "date", "event_format", "category", "budget_kzt"} {
		raw, ok := fields[key]
		if !ok || bytes.Equal(raw, []byte("null")) {
			errs = append(errs, domain.FieldError{Field: key, Code: "REQUIRED", Message: "Укажите обязательное поле."})
		}
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ptr, ok := known[key]
		if !ok {
			errs = append(errs, domain.FieldError{Field: key, Code: "UNKNOWN_FIELD", Message: "Неизвестное поле."})
			continue
		}
		if err = json.Unmarshal(fields[key], ptr); err != nil {
			errs = append(errs, domain.FieldError{Field: key, Code: "INVALID_TYPE", Message: "Неверный тип или недопустимый размер числа."})
		}
	}
	return r, errs, nil
}
