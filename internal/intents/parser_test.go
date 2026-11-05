package intents

import (
	"testing"

	"vyapari-voice-copilot/internal/domain"
)

func products() []domain.Product {
	return []domain.Product{
		{ID: "rice", Name: "Basmati Rice", Aliases: []string{"basmati", "rice", "బాస్మతి రైస్"}},
		{ID: "oil", Name: "Sunflower Oil", Aliases: []string{"oil", "నూనె"}},
	}
}

func TestDetectTeluguMixedStock(t *testing.T) {
	result := Detect("Basmati rice stock entha undi?", products())
	if result.Type != StockCheck {
		t.Fatalf("type=%s", result.Type)
	}
	if result.ProductID != "rice" {
		t.Fatalf("product=%s", result.ProductID)
	}
	if result.Language != "te-IN" {
		t.Fatalf("language=%s", result.Language)
	}
}

func TestDetectSaleQuantity(t *testing.T) {
	result := Detect("Basmati rice rendu packets sale ayindi", products())
	if result.Type != RecordSale {
		t.Fatalf("type=%s", result.Type)
	}
	if result.Quantity != 2 {
		t.Fatalf("quantity=%d", result.Quantity)
	}
}

func TestDetectReorderBeforeStock(t *testing.T) {
	result := Detect("low stock reorder list cheppu", products())
	if result.Type != LowStock && result.Type != ReorderPlan {
		t.Fatalf("type=%s", result.Type)
	}
}

func TestFuzzyProduct(t *testing.T) {
	result := Detect("basmti stock how many", products())
	if result.ProductID != "rice" {
		t.Fatalf("product=%s", result.ProductID)
	}
}
