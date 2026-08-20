package integration

import (
	"testing"
	"time"
)

func adversarialValidTransaction() CanonicalTransaction {
	return CanonicalTransaction{
		CompanyID:    "company",
		ConnectionID: "connection",
		Provider:     "poster",
		ExternalID:   "receipt-attack",
		Status:       "completed",
		OccurredAt:   time.Now(),
		GrossAmount:  1000,
		NetAmount:    1000,
		Currency:     "KZT",
	}
}

func TestAdversarialValidationRejectsNegativePayment(t *testing.T) {
	in := adversarialValidTransaction()
	in.Payments = []CanonicalPayment{{Type: "card", Status: "captured", Amount: -1000, OccurredAt: in.OccurredAt}}
	if err := validateTransaction(in); err == nil {
		t.Fatal("canonical transaction validation accepted a negative payment")
	}
}

func TestAdversarialValidationRejectsImpossibleTransactionTotals(t *testing.T) {
	in := adversarialValidTransaction()
	in.GrossAmount = 100
	in.DiscountAmount = 10
	in.NetAmount = 1000
	if err := validateTransaction(in); err == nil {
		t.Fatal("canonical transaction validation accepted net amount greater than gross amount")
	}
}

func TestAdversarialValidationRejectsInvalidPaymentState(t *testing.T) {
	in := adversarialValidTransaction()
	in.Payments = []CanonicalPayment{{Type: "attacker-type", Status: "invented", Amount: 1000}}
	if err := validateTransaction(in); err == nil {
		t.Fatal("canonical transaction validation accepted an unknown payment type/status and zero occurredAt")
	}
}

func TestAdversarialValidationRejectsNegativeItemCost(t *testing.T) {
	in := adversarialValidTransaction()
	negative := -50.0
	in.Items = []CanonicalItem{{Name: "Item", Quantity: 1, UnitPrice: 1000, GrossAmount: 1000, NetAmount: 1000, CostAmount: &negative}}
	if err := validateTransaction(in); err == nil {
		t.Fatal("canonical transaction validation accepted a negative item cost")
	}
}
