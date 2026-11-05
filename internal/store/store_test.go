package store

import (
	"errors"
	"testing"
	"time"
)

func TestRecordSaleAndUndo(t *testing.T) {
	s, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.Local)
	products := s.Products()
	if len(products) == 0 {
		t.Fatal("expected seeded products")
	}
	before := products[0].Stock

	updated, _, err := s.RecordSale(products[0].ID, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Stock != before-2 {
		t.Fatalf("stock=%d want=%d", updated.Stock, before-2)
	}

	if _, err := s.UndoLast(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	afterUndo := s.Products()[0].Stock
	if afterUndo != before {
		t.Fatalf("stock after undo=%d want=%d", afterUndo, before)
	}
}

func TestInsufficientStock(t *testing.T) {
	s, _ := Open("")
	product := s.Products()[0]
	_, _, err := s.RecordSale(product.ID, product.Stock+1, time.Now())
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestDashboardHasReorderPlan(t *testing.T) {
	s, _ := Open("")
	dashboard := s.Dashboard(time.Now())
	if dashboard.Metrics.LowStockCount == 0 {
		t.Fatal("expected low-stock products")
	}
	if len(dashboard.ReorderPlan) == 0 {
		t.Fatal("expected reorder recommendations")
	}
	if len(dashboard.SalesTrend) != 7 {
		t.Fatalf("sales trend length=%d want=7", len(dashboard.SalesTrend))
	}
}
