package dto_test

import (
	"encoding/json"
	"testing"

	"billing-payment-api/internal/dto"
)

func TestTHBJSON(t *testing.T) {
	for _, tc := range []struct {
		input, output string
		satang        dto.Amount
	}{
		{"0", "0.00", 0}, {"599", "599.00", 59900}, {"0.29", "0.29", 29},
		{"1800", "1800.00", 180000}, {"92233720368547758.07", "92233720368547758.07", 9223372036854775807},
	} {
		var amount dto.Amount
		if err := json.Unmarshal([]byte(tc.input), &amount); err != nil || amount != tc.satang {
			t.Fatalf("parse %s: %d %v", tc.input, amount, err)
		}
		data, err := json.Marshal(amount)
		if err != nil || string(data) != tc.output {
			t.Fatalf("serialize %s: %s %v", tc.input, data, err)
		}
	}
	for _, input := range []string{`"599"`, `null`, `true`, `-1`, `0.001`, `92233720368547758.08`} {
		var amount dto.Amount
		if err := json.Unmarshal([]byte(input), &amount); err == nil {
			t.Fatalf("accepted invalid THB: %s", input)
		}
	}
}
