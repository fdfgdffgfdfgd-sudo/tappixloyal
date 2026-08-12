package httpapi

import "testing"

func TestCanonicalLoyaltyState(t *testing.T) {
	stamp := canonicalLoyaltyState(map[string]any{"loyaltyMode": "stamps", "stampsTarget": float64(6), "stampReward": "Чистка"}, 20, 1, 20)
	if stamp["remaining"] != 5 || stamp["progress"] != 1 || stamp["rewardTitle"] != "Чистка" {
		t.Fatalf("unexpected stamp state: %#v", stamp)
	}
	eligible := canonicalLoyaltyState(map[string]any{"loyaltyMode": "stamps", "stampsTarget": float64(6)}, 20, 6, 20)
	if eligible["eligible"] != true || eligible["remaining"] != 0 {
		t.Fatalf("completed card is not eligible: %#v", eligible)
	}
	discount := canonicalLoyaltyState(map[string]any{"loyaltyMode": "discount", "discountStart": float64(3), "discountStep": float64(2), "discountMax": float64(15), "visitsPerStep": float64(3)}, 0, 9, 0)
	if discount["discountPercent"] != 9 {
		t.Fatalf("unexpected discount state: %#v", discount)
	}
}
