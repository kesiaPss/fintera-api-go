package models

import "testing"

func TestPaymentIsCancelled(t *testing.T) {
	payment := &Payment{Status: PaymentStatusCancelled}
	if !payment.IsCancelled() {
		t.Fatal("cancelled payment must report IsCancelled")
	}

	for _, status := range []string{
		PaymentStatusPending,
		PaymentStatusSubmitted,
		PaymentStatusPaid,
		PaymentStatusRejected,
		PaymentStatusReadjustment,
	} {
		payment.Status = status
		if payment.IsCancelled() {
			t.Errorf("IsCancelled(%q) = true, want false", status)
		}
	}
}
