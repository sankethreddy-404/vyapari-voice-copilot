package store

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"vyapari-voice-copilot/internal/domain"
)

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrInvalidQuantity   = errors.New("quantity must be greater than zero")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrNothingToUndo     = errors.New("there is no recent action to undo")
)

type Store struct {
	mu    sync.RWMutex
	path  string
	state domain.State
}

func Open(path string) (*Store, error) {
	s := &Store{path: path}
	now := time.Now()

	if strings.TrimSpace(path) == "" {
		s.state = seedState(now)
		return s, nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		s.state = seedState(now)
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read data file: %w", err)
	}

	if err := json.Unmarshal(data, &s.state); err != nil || len(s.state.Products) == 0 {
		backup := fmt.Sprintf("%s.broken-%d", path, now.Unix())
		_ = os.Rename(path, backup)
		s.state = seedState(now)
		if saveErr := s.saveLocked(); saveErr != nil {
			return nil, fmt.Errorf("recover data file: %w", saveErr)
		}
		return s, nil
	}

	return s, nil
}

func (s *Store) Products() []domain.Product {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneProducts(s.state.Products)
}

func (s *Store) Dashboard(now time.Time) domain.Dashboard {
	s.mu.RLock()
	defer s.mu.RUnlock()

	products := cloneProducts(s.state.Products)
	activities := cloneActivities(s.state.Activities)
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].CreatedAt.After(activities[j].CreatedAt)
	})
	if len(activities) > 12 {
		activities = activities[:12]
	}

	return domain.Dashboard{
		GeneratedAt:    now,
		Metrics:        metricsFor(s.state, now),
		Products:       products,
		ReorderPlan:    reorderPlanFor(s.state, now),
		RecentActivity: activities,
		SalesTrend:     trendFor(s.state, now, 7),
		TopProducts:    topProductsFor(s.state, now, 7, 5),
	}
}

func (s *Store) RecordSale(productID string, quantity int, now time.Time) (domain.Product, domain.Sale, error) {
	if quantity <= 0 {
		return domain.Product{}, domain.Sale{}, ErrInvalidQuantity
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	before := cloneState(s.state)
	index := productIndex(s.state.Products, productID)
	if index < 0 {
		return domain.Product{}, domain.Sale{}, ErrProductNotFound
	}
	product := &s.state.Products[index]
	if product.Stock < quantity {
		return domain.Product{}, domain.Sale{}, fmt.Errorf("%w: only %d %s available", ErrInsufficientStock, product.Stock, product.Unit)
	}

	product.Stock -= quantity
	product.UpdatedAt = now
	sale := domain.Sale{
		ID:          domain.NewID("sale"),
		ProductID:   product.ID,
		ProductName: product.Name,
		Quantity:    quantity,
		UnitPrice:   product.SellPrice,
		CostPrice:   product.CostPrice,
		Total:       float64(quantity) * product.SellPrice,
		Profit:      float64(quantity) * (product.SellPrice - product.CostPrice),
		CreatedAt:   now,
	}
	s.state.Sales = append(s.state.Sales, sale)
	s.state.Movements = append(s.state.Movements, domain.Movement{
		ID:          domain.NewID("move"),
		ProductID:   product.ID,
		ProductName: product.Name,
		Kind:        "sale",
		Delta:       -quantity,
		SaleID:      sale.ID,
		Note:        "Voice or dashboard sale",
		CreatedAt:   now,
	})
	s.addActivityLocked("sale", fmt.Sprintf("Recorded sale: %d %s of %s", quantity, product.Unit, product.Name), now, map[string]any{
		"productId": product.ID,
		"quantity":  quantity,
		"total":     sale.Total,
	})

	s.finishMutationLocked(now)
	if err := s.saveLocked(); err != nil {
		s.state = before
		return domain.Product{}, domain.Sale{}, err
	}
	return *product, sale, nil
}

func (s *Store) Restock(productID string, quantity int, note string, now time.Time) (domain.Product, error) {
	if quantity <= 0 {
		return domain.Product{}, ErrInvalidQuantity
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	before := cloneState(s.state)
	index := productIndex(s.state.Products, productID)
	if index < 0 {
		return domain.Product{}, ErrProductNotFound
	}
	product := &s.state.Products[index]
	product.Stock += quantity
	product.UpdatedAt = now
	if strings.TrimSpace(note) == "" {
		note = "Stock received"
	}
	s.state.Movements = append(s.state.Movements, domain.Movement{
		ID:          domain.NewID("move"),
		ProductID:   product.ID,
		ProductName: product.Name,
		Kind:        "restock",
		Delta:       quantity,
		Note:        note,
		CreatedAt:   now,
	})
	s.addActivityLocked("restock", fmt.Sprintf("Restocked %s by %d %s", product.Name, quantity, product.Unit), now, map[string]any{
		"productId": product.ID,
		"quantity":  quantity,
	})

	s.finishMutationLocked(now)
	if err := s.saveLocked(); err != nil {
		s.state = before
		return domain.Product{}, err
	}
	return *product, nil
}

func (s *Store) AddProduct(product domain.Product, now time.Time) (domain.Product, error) {
	name := strings.TrimSpace(product.Name)
	if name == "" {
		return domain.Product{}, errors.New("product name is required")
	}
	if product.Stock < 0 || product.LowStockThreshold < 0 || product.TargetStock <= 0 || product.CostPrice < 0 || product.SellPrice <= 0 {
		return domain.Product{}, errors.New("invalid product values")
	}
	if product.Unit == "" {
		product.Unit = "units"
	}
	if product.Category == "" {
		product.Category = "General"
	}
	if product.TargetStock <= product.LowStockThreshold {
		product.TargetStock = product.LowStockThreshold * 3
		if product.TargetStock == 0 {
			product.TargetStock = 20
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.state.Products {
		if strings.EqualFold(existing.Name, name) {
			return domain.Product{}, errors.New("a product with this name already exists")
		}
	}

	before := cloneState(s.state)
	product.ID = domain.NewID("prd")
	product.Name = name
	product.Aliases = cleanAliases(append(product.Aliases, name))
	product.CreatedAt = now
	product.UpdatedAt = now
	s.state.Products = append(s.state.Products, product)
	s.addActivityLocked("product", fmt.Sprintf("Added product: %s", product.Name), now, map[string]any{"productId": product.ID})
	s.finishMutationLocked(now)

	if err := s.saveLocked(); err != nil {
		s.state = before
		return domain.Product{}, err
	}
	return product, nil
}

func (s *Store) UndoLast(now time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	before := cloneState(s.state)
	for i := len(s.state.Movements) - 1; i >= 0; i-- {
		movement := &s.state.Movements[i]
		if movement.Undone || (movement.Kind != "sale" && movement.Kind != "restock") {
			continue
		}
		productIdx := productIndex(s.state.Products, movement.ProductID)
		if productIdx < 0 {
			continue
		}
		product := &s.state.Products[productIdx]

		switch movement.Kind {
		case "sale":
			product.Stock += -movement.Delta
			for saleIdx := range s.state.Sales {
				if s.state.Sales[saleIdx].ID == movement.SaleID {
					s.state.Sales[saleIdx].Reversed = true
					break
				}
			}
		case "restock":
			if product.Stock < movement.Delta {
				return "", errors.New("cannot undo restock because some of that stock has already been sold")
			}
			product.Stock -= movement.Delta
		}

		product.UpdatedAt = now
		movement.Undone = true
		message := fmt.Sprintf("Undid %s for %s", movement.Kind, product.Name)
		s.addActivityLocked("undo", message, now, map[string]any{"movementId": movement.ID})
		s.finishMutationLocked(now)
		if err := s.saveLocked(); err != nil {
			s.state = before
			return "", err
		}
		return message, nil
	}

	return "", ErrNothingToUndo
}

func (s *Store) Reset(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := cloneState(s.state)
	s.state = seedState(now)
	s.addActivityLocked("reset", "Demo data reset", now, nil)
	s.finishMutationLocked(now)
	if err := s.saveLocked(); err != nil {
		s.state = before
		return err
	}
	return nil
}

func (s *Store) ExportCSV(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"Product", "Category", "Stock", "Unit", "Low stock threshold", "Target stock", "Cost price", "Sell price", "Stock value"}); err != nil {
		return err
	}
	for _, p := range s.state.Products {
		row := []string{
			p.Name,
			p.Category,
			strconv.Itoa(p.Stock),
			p.Unit,
			strconv.Itoa(p.LowStockThreshold),
			strconv.Itoa(p.TargetStock),
			fmt.Sprintf("%.2f", p.CostPrice),
			fmt.Sprintf("%.2f", p.SellPrice),
			fmt.Sprintf("%.2f", float64(p.Stock)*p.CostPrice),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return writer.Error()
}

func (s *Store) addActivityLocked(kind, message string, now time.Time, metadata map[string]any) {
	s.state.Activities = append(s.state.Activities, domain.Activity{
		ID:        domain.NewID("act"),
		Kind:      kind,
		Message:   message,
		CreatedAt: now,
		Metadata:  metadata,
	})
}

func (s *Store) finishMutationLocked(now time.Time) {
	s.state.Version = 1
	s.state.UpdatedAt = now
	if len(s.state.Sales) > 1000 {
		s.state.Sales = append([]domain.Sale(nil), s.state.Sales[len(s.state.Sales)-1000:]...)
	}
	if len(s.state.Movements) > 500 {
		s.state.Movements = append([]domain.Movement(nil), s.state.Movements[len(s.state.Movements)-500:]...)
	}
	if len(s.state.Activities) > 250 {
		s.state.Activities = append([]domain.Activity(nil), s.state.Activities[len(s.state.Activities)-250:]...)
	}
}

func (s *Store) saveLocked() error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode data: %w", err)
	}
	temp := s.path + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return fmt.Errorf("write temporary data file: %w", err)
	}
	_ = os.Remove(s.path)
	if err := os.Rename(temp, s.path); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("replace data file: %w", err)
	}
	return nil
}

func metricsFor(state domain.State, now time.Time) domain.Metrics {
	var metrics domain.Metrics
	for _, product := range state.Products {
		metrics.InventoryValue += float64(product.Stock) * product.CostPrice
		if product.Stock <= product.LowStockThreshold {
			metrics.LowStockCount++
		}
	}
	for _, sale := range state.Sales {
		if sale.Reversed || !sameDay(sale.CreatedAt, now) {
			continue
		}
		metrics.TodayRevenue += sale.Total
		metrics.TodayProfit += sale.Profit
		metrics.TodayOrders++
		metrics.TodayItemsSold += sale.Quantity
	}
	return metrics
}

func trendFor(state domain.State, now time.Time, days int) []domain.DailyMetric {
	if days <= 0 {
		return nil
	}
	result := make([]domain.DailyMetric, days)
	indexByDate := make(map[string]int, days)
	for i := 0; i < days; i++ {
		day := startOfDay(now).AddDate(0, 0, -(days - 1 - i))
		key := day.Format("2006-01-02")
		result[i] = domain.DailyMetric{Date: key}
		indexByDate[key] = i
	}
	for _, sale := range state.Sales {
		if sale.Reversed {
			continue
		}
		key := sale.CreatedAt.In(now.Location()).Format("2006-01-02")
		index, ok := indexByDate[key]
		if !ok {
			continue
		}
		result[index].Revenue += sale.Total
		result[index].Profit += sale.Profit
		result[index].Orders++
		result[index].Items += sale.Quantity
	}
	return result
}

func topProductsFor(state domain.State, now time.Time, days, limit int) []domain.ProductPerformance {
	cutoff := startOfDay(now).AddDate(0, 0, -(days - 1))
	byProduct := make(map[string]*domain.ProductPerformance)
	for _, sale := range state.Sales {
		if sale.Reversed || sale.CreatedAt.Before(cutoff) {
			continue
		}
		current := byProduct[sale.ProductID]
		if current == nil {
			current = &domain.ProductPerformance{ProductID: sale.ProductID, ProductName: sale.ProductName}
			byProduct[sale.ProductID] = current
		}
		current.Quantity += sale.Quantity
		current.Revenue += sale.Total
	}
	result := make([]domain.ProductPerformance, 0, len(byProduct))
	for _, value := range byProduct {
		result = append(result, *value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Quantity == result[j].Quantity {
			return result[i].Revenue > result[j].Revenue
		}
		return result[i].Quantity > result[j].Quantity
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func reorderPlanFor(state domain.State, now time.Time) []domain.ReorderItem {
	cutoff := startOfDay(now).AddDate(0, 0, -6)
	sold := make(map[string]int)
	for _, sale := range state.Sales {
		if !sale.Reversed && !sale.CreatedAt.Before(cutoff) {
			sold[sale.ProductID] += sale.Quantity
		}
	}

	result := make([]domain.ReorderItem, 0)
	for _, product := range state.Products {
		average := float64(sold[product.ID]) / 7.0
		daysOfCover := 999.0
		if average > 0 {
			daysOfCover = float64(product.Stock) / average
		}
		needsReorder := product.Stock <= product.LowStockThreshold || daysOfCover < 5
		if !needsReorder {
			continue
		}

		dynamicTarget := int(math.Ceil(average * 14))
		target := product.TargetStock
		if dynamicTarget > target {
			target = dynamicTarget
		}
		recommended := target - product.Stock
		if recommended < 1 {
			recommended = 1
		}

		priority := "medium"
		switch {
		case product.Stock == 0 || daysOfCover < 1.5:
			priority = "critical"
		case product.Stock <= max(1, product.LowStockThreshold/2) || daysOfCover < 3:
			priority = "high"
		}

		reason := fmt.Sprintf("Stock is %d; target is %d", product.Stock, target)
		if average > 0 {
			reason = fmt.Sprintf("About %.1f days of stock remain at %.1f units/day", daysOfCover, average)
		}
		result = append(result, domain.ReorderItem{
			ProductID:      product.ID,
			ProductName:    product.Name,
			CurrentStock:   product.Stock,
			RecommendedQty: recommended,
			EstimatedCost:  float64(recommended) * product.CostPrice,
			Priority:       priority,
			AverageDaily:   average,
			DaysOfCover:    daysOfCover,
			Reason:         reason,
		})
	}

	rank := map[string]int{"critical": 0, "high": 1, "medium": 2}
	sort.Slice(result, func(i, j int) bool {
		if rank[result[i].Priority] == rank[result[j].Priority] {
			return result[i].DaysOfCover < result[j].DaysOfCover
		}
		return rank[result[i].Priority] < rank[result[j].Priority]
	})
	return result
}

func seedState(now time.Time) domain.State {
	product := func(name, category, unit string, stock, low, target int, cost, sell float64, aliases ...string) domain.Product {
		return domain.Product{
			ID:                domain.NewID("prd"),
			Name:              name,
			Aliases:           cleanAliases(append(aliases, name)),
			Category:          category,
			Unit:              unit,
			Stock:             stock,
			LowStockThreshold: low,
			TargetStock:       target,
			CostPrice:         cost,
			SellPrice:         sell,
			CreatedAt:         now.AddDate(0, -1, 0),
			UpdatedAt:         now,
		}
	}

	products := []domain.Product{
		product("Basmati Rice", "Grains", "packets", 18, 8, 40, 70, 90, "basmati", "rice", "బాస్మతి", "బియ్యం", "బాస్మతి రైస్"),
		product("Sunflower Oil", "Cooking", "bottles", 6, 8, 30, 110, 135, "oil", "sunflower", "నూనె", "ఆయిల్", "సన్ ఫ్లవర్ ఆయిల్"),
		product("Wheat Flour", "Grains", "packets", 22, 10, 35, 42, 55, "atta", "flour", "wheat", "పిండి", "గోధుమ పిండి"),
		product("Sugar", "Groceries", "packets", 7, 8, 30, 38, 48, "sugar", "చక్కెర", "పంచదార"),
		product("Milk", "Dairy", "packets", 12, 6, 25, 25, 30, "milk", "పాలు"),
		product("Toor Dal", "Pulses", "packets", 5, 7, 25, 120, 145, "dal", "toor", "పప్పు", "కందిపప్పు"),
		product("Bath Soap", "Personal Care", "bars", 30, 10, 50, 22, 30, "soap", "సబ్బు"),
		product("Biscuits", "Snacks", "packets", 15, 8, 35, 10, 15, "biscuit", "biscuits", "బిస్కెట్లు"),
		product("Iodised Salt", "Groceries", "packets", 20, 8, 30, 18, 24, "salt", "ఉప్పు"),
	}

	byName := make(map[string]domain.Product, len(products))
	for _, p := range products {
		byName[p.Name] = p
	}

	var sales []domain.Sale
	addSale := func(name string, qty, daysAgo, hour int) {
		p := byName[name]
		at := startOfDay(now).AddDate(0, 0, -daysAgo).Add(time.Duration(hour) * time.Hour)
		sales = append(sales, domain.Sale{
			ID:          domain.NewID("sale"),
			ProductID:   p.ID,
			ProductName: p.Name,
			Quantity:    qty,
			UnitPrice:   p.SellPrice,
			CostPrice:   p.CostPrice,
			Total:       float64(qty) * p.SellPrice,
			Profit:      float64(qty) * (p.SellPrice - p.CostPrice),
			CreatedAt:   at,
		})
	}

	addSale("Basmati Rice", 3, 0, 9)
	addSale("Milk", 5, 0, 10)
	addSale("Biscuits", 4, 0, 12)
	addSale("Sunflower Oil", 2, 0, 14)
	addSale("Bath Soap", 3, 0, 16)
	addSale("Basmati Rice", 5, 1, 11)
	addSale("Sugar", 4, 1, 15)
	addSale("Milk", 7, 1, 17)
	addSale("Wheat Flour", 6, 2, 10)
	addSale("Toor Dal", 3, 2, 14)
	addSale("Biscuits", 8, 3, 13)
	addSale("Basmati Rice", 4, 3, 18)
	addSale("Sunflower Oil", 4, 4, 12)
	addSale("Milk", 6, 4, 17)
	addSale("Bath Soap", 5, 5, 11)
	addSale("Wheat Flour", 4, 5, 16)
	addSale("Basmati Rice", 6, 6, 10)
	addSale("Sugar", 5, 6, 15)

	activities := []domain.Activity{
		{ID: domain.NewID("act"), Kind: "insight", Message: "Sunflower Oil and Toor Dal need attention", CreatedAt: now.Add(-45 * time.Minute)},
		{ID: domain.NewID("act"), Kind: "sale", Message: "Recorded recent demo sales", CreatedAt: now.Add(-2 * time.Hour)},
		{ID: domain.NewID("act"), Kind: "system", Message: "Vyapari Voice Copilot is ready", CreatedAt: now.Add(-3 * time.Hour)},
	}

	return domain.State{
		Version:    1,
		Products:   products,
		Sales:      sales,
		Movements:  []domain.Movement{},
		Activities: activities,
		UpdatedAt:  now,
	}
}

func productIndex(products []domain.Product, id string) int {
	for i := range products {
		if products[i].ID == id {
			return i
		}
	}
	return -1
}

func sameDay(a, b time.Time) bool {
	a = a.In(b.Location())
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func startOfDay(value time.Time) time.Time {
	value = value.In(value.Location())
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func cleanAliases(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func cloneState(state domain.State) domain.State {
	result := state
	result.Products = cloneProducts(state.Products)
	result.Sales = append([]domain.Sale(nil), state.Sales...)
	result.Movements = append([]domain.Movement(nil), state.Movements...)
	result.Activities = cloneActivities(state.Activities)
	return result
}

func cloneProducts(products []domain.Product) []domain.Product {
	result := append([]domain.Product(nil), products...)
	for i := range result {
		result[i].Aliases = append([]string(nil), result[i].Aliases...)
	}
	return result
}

func cloneActivities(activities []domain.Activity) []domain.Activity {
	result := append([]domain.Activity(nil), activities...)
	for i := range result {
		if result[i].Metadata != nil {
			metadata := make(map[string]any, len(result[i].Metadata))
			for key, value := range result[i].Metadata {
				metadata[key] = value
			}
			result[i].Metadata = metadata
		}
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
