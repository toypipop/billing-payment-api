package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"billing-payment-api/internal/dto"
)

func TestPaymentValidation(t *testing.T) {
	positive, zero, negative := dto.Amount(1), dto.Amount(0), dto.Amount(-1)
	for _, req := range []dto.CreatePaymentRequest{
		{Unit: "", AmountTHB: &positive},
		{Unit: "  ", AmountTHB: &positive},
		{Unit: strings.Repeat("A", 51), AmountTHB: &positive},
		{Unit: "A101"},
		{Unit: "A101", AmountTHB: &zero},
		{Unit: "A101", AmountTHB: &negative},
	} {
		if _, err := NewPaymentService(nil).Create(context.Background(), req); !errors.Is(err, ErrInvalidPayment) {
			t.Fatalf("expected invalid payment, got %v", err)
		}
	}
}
