package intents

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"vyapari-voice-copilot/internal/domain"
)

const (
	Unknown       = "unknown"
	Greeting      = "greeting"
	Help          = "help"
	StockCheck    = "stock_check"
	InventoryList = "inventory_list"
	RecordSale    = "record_sale"
	Restock       = "restock"
	LowStock      = "low_stock"
	SalesSummary  = "sales_summary"
	ProfitSummary = "profit_summary"
	ReorderPlan   = "reorder_plan"
	TopProducts   = "top_products"
	Briefing      = "business_briefing"
	Undo          = "undo"
)

type Result struct {
	Type        string  `json:"type"`
	ProductID   string  `json:"productId,omitempty"`
	ProductName string  `json:"productName,omitempty"`
	Quantity    int     `json:"quantity,omitempty"`
	Language    string  `json:"language"`
	Confidence  float64 `json:"confidence"`
	Original    string  `json:"original"`
}

var numberPattern = regexp.MustCompile(`\b\d+\b`)

func Detect(text string, products []domain.Product) Result {
	normalized := normalize(text)
	language := DetectLanguage(text)
	result := Result{
		Type:       Unknown,
		Language:   language,
		Confidence: 0.35,
		Original:   strings.TrimSpace(text),
	}

	switch {
	case containsAny(normalized, "undo", "cancel last", "reverse last", "వెనక్కి", "రద్దు చేయి", "చివరిది తీసేయి"):
		result.Type, result.Confidence = Undo, 0.98
	case containsAny(normalized, "help", "commands", "what can you do", "సహాయం", "ఏం చేయగలవు", "em cheyagalavu"):
		result.Type, result.Confidence = Help, 0.98
	case containsAny(normalized, "how is my shop", "business briefing", "daily briefing", "shop summary", "complete summary", "వ్యాపారం ఎలా ఉంది", "షాప్ ఎలా ఉంది", "motham summary"):
		result.Type, result.Confidence = Briefing, 0.96
	case containsAny(normalized, "low stock", "running low", "తక్కువ స్టాక్", "తక్కువగా ఉన్నాయి", "low ga", "stock thakkuva"):
		result.Type, result.Confidence = LowStock, 0.97
	case containsAny(normalized, "reorder", "re order", "purchase list", "order list", "కొనాల్సిన", "తెప్పించాలి", "malli order", "reorder cheppu"):
		result.Type, result.Confidence = ReorderPlan, 0.96
	case containsAny(normalized, "top product", "top products", "best selling", "fast moving", "most sold", "ఎక్కువ అమ్ముడైన", "బాగా అమ్ముడైన"):
		result.Type, result.Confidence = TopProducts, 0.96
	case containsAny(normalized, "profit", "margin", "లాభం", "labham"):
		result.Type, result.Confidence = ProfitSummary, 0.95
	case containsAny(normalized, "today sales", "today's sales", "sales today", "revenue", "ఈ రోజు అమ్మకాలు", "ఈరోజు అమ్మకాలు", "ee roju sales", "ivala sales", "ivvala sales"):
		result.Type, result.Confidence = SalesSummary, 0.95
	case isRestock(normalized):
		result.Type, result.Confidence = Restock, 0.91
	case isSale(normalized):
		result.Type, result.Confidence = RecordSale, 0.92
	case containsAny(normalized, "inventory list", "show inventory", "all products", "list products", "స్టాక్ లిస్ట్", "అన్ని ప్రొడక్ట్స్"):
		result.Type, result.Confidence = InventoryList, 0.94
	case containsAny(normalized, "stock", "available", "how many", "ఎంత ఉంది", "ఎన్ని ఉన్నాయి", "స్టాక్ ఎంత", "entha undi", "enni unnayi"):
		result.Type, result.Confidence = StockCheck, 0.88
	case containsAny(normalized, "hello", "hi", "hey", "namaste", "నమస్తే", "హలో"):
		result.Type, result.Confidence = Greeting, 0.90
	}

	if product, score := matchProduct(normalized, products); product != nil {
		result.ProductID = product.ID
		result.ProductName = product.Name
		if score > result.Confidence {
			result.Confidence = math.Min(0.99, score)
		}
	}

	if result.Type == RecordSale || result.Type == Restock {
		result.Quantity = parseQuantity(normalized)
		if result.Quantity <= 0 {
			result.Quantity = 1
		}
	}

	return result
}

func DetectLanguage(text string) string {
	for _, r := range text {
		switch {
		case r >= 0x0C00 && r <= 0x0C7F:
			return "te-IN"
		case r >= 0x0900 && r <= 0x097F:
			return "hi-IN"
		}
	}
	normalized := normalize(text)
	if containsAny(normalized,
		"entha", "undi", "unnayi", "cheppu", "rendu", "moodu", "nalugu", "aidu", "ivala", "ivvala", "ee roju", "ammakam", "ayindi", "stock thakkuva") {
		return "te-IN"
	}
	if containsAny(normalized, "kitna", "hai", "batao", "aaj", "becha", "do packet", "teen packet") {
		return "hi-en"
	}
	return "en-IN"
}

func isSale(text string) bool {
	return containsAny(text,
		"sale", "sold", "sell", "customer bought", "order completed",
		"అమ్మకం", "అమ్మాను", "అమ్ముడైంది", "కొన్నాడు", "కొన్నారు",
		"sale ayindi", "ammakam", "ammindi")
}

func isRestock(text string) bool {
	return containsAny(text,
		"restock", "re stock", "add stock", "stock received", "received stock", "new stock",
		"స్టాక్ వచ్చింది", "స్టాక్ చేర్చు", "కొత్త స్టాక్", "వచ్చాయి", "stock vachindi", "stock add")
}

func matchProduct(text string, products []domain.Product) (*domain.Product, float64) {
	type candidate struct {
		product domain.Product
		alias   string
	}
	candidates := make([]candidate, 0)
	for _, product := range products {
		aliases := append([]string{product.Name}, product.Aliases...)
		for _, alias := range aliases {
			alias = normalize(alias)
			if alias != "" {
				candidates = append(candidates, candidate{product: product, alias: alias})
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return len([]rune(candidates[i].alias)) > len([]rune(candidates[j].alias)) })

	for _, candidate := range candidates {
		if containsPhrase(text, candidate.alias) {
			score := 0.90 + math.Min(0.08, float64(len([]rune(candidate.alias)))/200)
			product := candidate.product
			return &product, score
		}
	}

	words := strings.Fields(text)
	bestScore := 0.0
	var best *domain.Product
	for _, candidate := range candidates {
		aliasWords := strings.Fields(candidate.alias)
		if len(aliasWords) != 1 {
			continue
		}
		for _, word := range words {
			if len([]rune(word)) < 4 || len([]rune(candidate.alias)) < 4 {
				continue
			}
			distance := levenshtein(word, candidate.alias)
			longest := max(len([]rune(word)), len([]rune(candidate.alias)))
			score := 1 - float64(distance)/float64(longest)
			if score >= 0.72 && score > bestScore {
				product := candidate.product
				best = &product
				bestScore = score
			}
		}
	}
	if best != nil {
		return best, 0.72 + bestScore*0.15
	}
	return nil, 0
}

func parseQuantity(text string) int {
	if raw := numberPattern.FindString(text); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			return value
		}
	}

	values := map[string]int{
		"one": 1, "a": 1, "single": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11,
		"twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16,
		"seventeen": 17, "eighteen": 18, "nineteen": 19, "twenty": 20,
		"okati": 1, "rendu": 2, "moodu": 3, "nalugu": 4, "aidu": 5, "aaru": 6,
		"edu": 7, "enimidi": 8, "tommidi": 9, "padi": 10,
		"ఒకటి": 1, "రెండు": 2, "మూడు": 3, "నాలుగు": 4, "ఐదు": 5,
		"ఆరు": 6, "ఏడు": 7, "ఎనిమిది": 8, "తొమ్మిది": 9, "పది": 10,
		"ek": 1, "do": 2, "teen": 3, "char": 4, "paanch": 5, "chhe": 6,
		"saat": 7, "aath": 8, "nau": 9, "das": 10,
	}
	tens := map[string]int{"thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90}

	fields := strings.Fields(text)
	total := 0
	found := false
	for _, field := range fields {
		field = strings.Trim(field, " ,.!?;:-")
		if value, ok := values[field]; ok {
			total += value
			found = true
			continue
		}
		if value, ok := tens[field]; ok {
			total += value
			found = true
		}
	}
	if found {
		return total
	}
	return 0
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastSpace := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if containsPhrase(text, normalize(value)) {
			return true
		}
	}
	return false
}

func containsPhrase(text, phrase string) bool {
	if phrase == "" {
		return false
	}
	return strings.Contains(" "+text+" ", " "+phrase+" ") || strings.Contains(text, phrase)
}

func levenshtein(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	previous := make([]int, len(br)+1)
	current := make([]int, len(br)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		current[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			current[j] = min(
				previous[j]+1,
				current[j-1]+1,
				previous[j-1]+cost,
			)
		}
		previous, current = current, previous
	}
	return previous[len(br)]
}

func min(values ...int) int {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
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
