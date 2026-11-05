package domain

import "time"

type Product struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Aliases           []string  `json:"aliases"`
	Category          string    `json:"category"`
	Unit              string    `json:"unit"`
	Stock             int       `json:"stock"`
	LowStockThreshold int       `json:"lowStockThreshold"`
	TargetStock       int       `json:"targetStock"`
	CostPrice         float64   `json:"costPrice"`
	SellPrice         float64   `json:"sellPrice"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type Sale struct {
	ID          string    `json:"id"`
	ProductID   string    `json:"productId"`
	ProductName string    `json:"productName"`
	Quantity    int       `json:"quantity"`
	UnitPrice   float64   `json:"unitPrice"`
	CostPrice   float64   `json:"costPrice"`
	Total       float64   `json:"total"`
	Profit      float64   `json:"profit"`
	CreatedAt   time.Time `json:"createdAt"`
	Reversed    bool      `json:"reversed"`
}

type Movement struct {
	ID          string    `json:"id"`
	ProductID   string    `json:"productId"`
	ProductName string    `json:"productName"`
	Kind        string    `json:"kind"`
	Delta       int       `json:"delta"`
	SaleID      string    `json:"saleId,omitempty"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"createdAt"`
	Undone      bool      `json:"undone"`
}

type Activity struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Message   string         `json:"message"`
	CreatedAt time.Time      `json:"createdAt"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type State struct {
	Version    int        `json:"version"`
	Products   []Product  `json:"products"`
	Sales      []Sale     `json:"sales"`
	Movements  []Movement `json:"movements"`
	Activities []Activity `json:"activities"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type Metrics struct {
	TodayRevenue   float64 `json:"todayRevenue"`
	TodayProfit    float64 `json:"todayProfit"`
	TodayOrders    int     `json:"todayOrders"`
	TodayItemsSold int     `json:"todayItemsSold"`
	InventoryValue float64 `json:"inventoryValue"`
	LowStockCount  int     `json:"lowStockCount"`
}

type DailyMetric struct {
	Date    string  `json:"date"`
	Revenue float64 `json:"revenue"`
	Profit  float64 `json:"profit"`
	Orders  int     `json:"orders"`
	Items   int     `json:"items"`
}

type ProductPerformance struct {
	ProductID   string  `json:"productId"`
	ProductName string  `json:"productName"`
	Quantity    int     `json:"quantity"`
	Revenue     float64 `json:"revenue"`
}

type ReorderItem struct {
	ProductID      string  `json:"productId"`
	ProductName    string  `json:"productName"`
	CurrentStock   int     `json:"currentStock"`
	RecommendedQty int     `json:"recommendedQty"`
	EstimatedCost  float64 `json:"estimatedCost"`
	Priority       string  `json:"priority"`
	AverageDaily   float64 `json:"averageDailySales"`
	DaysOfCover    float64 `json:"daysOfCover"`
	Reason         string  `json:"reason"`
}

type Dashboard struct {
	GeneratedAt    time.Time            `json:"generatedAt"`
	Metrics        Metrics              `json:"metrics"`
	Products       []Product            `json:"products"`
	ReorderPlan    []ReorderItem        `json:"reorderPlan"`
	RecentActivity []Activity           `json:"recentActivity"`
	SalesTrend     []DailyMetric        `json:"salesTrend"`
	TopProducts    []ProductPerformance `json:"topProducts"`
}
