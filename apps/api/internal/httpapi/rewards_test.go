package httpapi

import (
	"testing"
	"time"
)

func TestEffectiveRewardStatus(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	cases := []struct {
		name     string
		status   string
		reserved *time.Time
		want     string
	}{
		{
			// The reward list shows this one as available. Refusing to hand it
			// over told staff it had "already been processed".
			name: "a reservation that ran out is available again", status: "reserved", reserved: &past, want: "available",
		},
		{name: "a live reservation still holds", status: "reserved", reserved: &future, want: "reserved"},
		{name: "a reservation without a deadline holds", status: "reserved", reserved: nil, want: "reserved"},
		{name: "expiring exactly now releases", status: "reserved", reserved: &now, want: "available"},
		{name: "redeemed stays redeemed", status: "redeemed", reserved: &past, want: "redeemed"},
		{name: "cancelled stays cancelled", status: "cancelled", reserved: &past, want: "cancelled"},
		{name: "available is untouched", status: "available", reserved: nil, want: "available"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := effectiveRewardStatus(c.status, c.reserved, now); got != c.want {
				t.Fatalf("effectiveRewardStatus(%q) = %q, want %q", c.status, got, c.want)
			}
		})
	}
}

func TestRemainingInventory(t *testing.T) {
	total := func(n int) *int { return &n }

	if got := remainingInventory(nil, 5); got != nil {
		t.Fatalf("unlimited stock should report nil, got %v", got)
	}
	if got := remainingInventory(total(10), 3); got != 7 {
		t.Fatalf("remaining = %v, want 7", got)
	}
	if got := remainingInventory(total(10), 10); got != 0 {
		t.Fatalf("exhausted stock should report 0, got %v", got)
	}
	// Issuing more than the stock must not report a negative count back to the
	// interface.
	if got := remainingInventory(total(3), 5); got != 0 {
		t.Fatalf("over-issued stock should report 0, got %v", got)
	}
}

func TestNormalizeDefinition(t *testing.T) {
	t.Run("fills in the defaults", func(t *testing.T) {
		in := rewardDefinitionInput{Name: "  Кофе  "}
		if err := normalizeDefinition(&in); err != nil {
			t.Fatalf("rejected: %v", err)
		}
		if in.Name != "Кофе" {
			t.Fatalf("name = %q, want trimmed", in.Name)
		}
		if in.RewardType != "gift" || in.ConfirmationMethod != "staff" {
			t.Fatalf("defaults not applied: %+v", in)
		}
	})

	t.Run("lowercases what the API compares", func(t *testing.T) {
		in := rewardDefinitionInput{Name: "Кофе", RewardType: "DISCOUNT", ConfirmationMethod: "Staff"}
		if err := normalizeDefinition(&in); err != nil {
			t.Fatalf("rejected: %v", err)
		}
		if in.RewardType != "discount" || in.ConfirmationMethod != "staff" {
			t.Fatalf("not lowercased: %+v", in)
		}
	})

	for _, c := range []struct {
		name string
		in   rewardDefinitionInput
	}{
		{"no name", rewardDefinitionInput{}},
		{"name is only spaces", rewardDefinitionInput{Name: "   "}},
		{"negative value", rewardDefinitionInput{Name: "Кофе", Value: -1}},
		{"negative cooldown", rewardDefinitionInput{Name: "Кофе", CooldownDays: -1}},
	} {
		t.Run("rejects "+c.name, func(t *testing.T) {
			in := c.in
			if normalizeDefinition(&in) == nil {
				t.Fatalf("accepted an invalid definition: %+v", c.in)
			}
		})
	}
}

func TestRewardCycle(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	if got := rewardCycle("calendar_month", now); got != "2026-08" {
		t.Fatalf("monthly cycle = %q", got)
	}
	if got := rewardCycle("calendar_year", now); got != "2026" {
		t.Fatalf("yearly cycle = %q", got)
	}
	// Anything unrecognised must fall back to a single lifetime bucket rather
	// than inventing a cycle that lets a reward be claimed again.
	if got := rewardCycle("", now); got != "lifetime" {
		t.Fatalf("default cycle = %q", got)
	}
	if got := rewardCycle("weekly", now); got != "lifetime" {
		t.Fatalf("unknown cycle = %q", got)
	}
}
