package api

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"vyapari-voice-copilot/internal/domain"
	"vyapari-voice-copilot/internal/gnani"
	"vyapari-voice-copilot/internal/intents"
	storepkg "vyapari-voice-copilot/internal/store"
)

type commandRequest struct {
	Text             string `json:"text"`
	ResponseLanguage string `json:"responseLanguage,omitempty"`
}

type commandResponse struct {
	Success     bool             `json:"success"`
	Intent      intents.Result   `json:"intent"`
	Message     string           `json:"message"`
	SpeechText  string           `json:"speechText"`
	Language    string           `json:"language"`
	Changed     bool             `json:"changed"`
	Dashboard   domain.Dashboard `json:"dashboard"`
	Suggestions []string         `json:"suggestions,omitempty"`
}

type ttsRequest struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Voice    string  `json:"voice,omitempty"`
	Model    string  `json:"model,omitempty"`
	Speed    float64 `json:"speed,omitempty"`
}

type productRequest struct {
	Name              string   `json:"name"`
	Aliases           []string `json:"aliases,omitempty"`
	Category          string   `json:"category"`
	Unit              string   `json:"unit"`
	Stock             int      `json:"stock"`
	LowStockThreshold int      `json:"lowStockThreshold"`
	TargetStock       int      `json:"targetStock"`
	CostPrice         float64  `json:"costPrice"`
	SellPrice         float64  `json:"sellPrice"`
}

type stockActionRequest struct {
	ProductID string `json:"productId"`
	Action    string `json:"action"`
	Quantity  int    `json:"quantity"`
	Note      string `json:"note,omitempty"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"status":     "ok",
		"version":    Version,
		"project":    "Vyapari Voice Copilot",
		"serverTime": time.Now(),
		"uptime":     time.Since(s.startedAt).Round(time.Second).String(),
		"gnani":      s.gnani.PublicConfig(),
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"dashboard": s.store.Dashboard(time.Now()),
	})
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var request commandRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Text = strings.TrimSpace(request.Text)
	if request.Text == "" {
		writeError(w, http.StatusBadRequest, "TEXT_REQUIRED", "Type or speak a command first", nil)
		return
	}

	products := s.store.Products()
	intent := intents.Detect(request.Text, products)
	language := responseLanguage(request.ResponseLanguage, intent.Language)
	response := commandResponse{
		Success:     true,
		Intent:      intent,
		Language:    language,
		Suggestions: defaultSuggestions(language),
	}

	english, telugu := "", ""
	switch intent.Type {
	case intents.Greeting:
		english = "Namaste! I can record sales, check stock, find low-stock items and prepare a reorder plan."
		telugu = "నమస్కారం! అమ్మకాలు నమోదు చేయడం, స్టాక్ చూడడం, తక్కువ స్టాక్ గుర్తించడం మరియు రీఆర్డర్ ప్లాన్ తయారు చేయడం నేను చేయగలను."

	case intents.Help:
		english = "Try: Basmati rice stock entha undi; Basmati rice rendu packets sale ayindi; low stock products cheppu; or today sales entha."
		telugu = "ఇలా అడగండి: Basmati rice stock entha undi; Basmati rice rendu packets sale ayindi; low stock products cheppu; లేదా today sales entha."

	case intents.InventoryList:
		names := make([]string, 0, len(products))
		for _, product := range products {
			names = append(names, fmt.Sprintf("%s %d", product.Name, product.Stock))
		}
		english = "Current inventory: " + strings.Join(names, ", ") + "."
		telugu = "ప్రస్తుత స్టాక్: " + strings.Join(names, ", ") + "."

	case intents.StockCheck:
		product, ok := productByID(products, intent.ProductID)
		if !ok {
			response.Success = false
			english = "I understood that you want a stock check, but I could not identify the product. Say a product name such as Basmati Rice, Oil, Sugar or Toor Dal."
			telugu = "మీరు స్టాక్ అడిగారని అర్థమైంది, కానీ ప్రొడక్ట్ పేరు గుర్తించలేకపోయాను. Basmati Rice, Oil, Sugar లేదా Toor Dal అని చెప్పండి."
			break
		}
		statusEN := "Stock is healthy."
		statusTE := "స్టాక్ సరిపోతుంది."
		if product.Stock <= product.LowStockThreshold {
			statusEN = "It is below the safe-stock level."
			statusTE = "ఇది సేఫ్ స్టాక్ స్థాయి కంటే తక్కువగా ఉంది."
		}
		english = fmt.Sprintf("%s has %d %s in stock. %s", product.Name, product.Stock, product.Unit, statusEN)
		telugu = fmt.Sprintf("%s స్టాక్‌లో %d %s ఉన్నాయి. %s", product.Name, product.Stock, product.Unit, statusTE)

	case intents.RecordSale:
		product, ok := productByID(products, intent.ProductID)
		if !ok {
			response.Success = false
			english = "I heard a sale command, but I could not identify the product. Please include the product name."
			telugu = "అమ్మకం కమాండ్ అర్థమైంది, కానీ ప్రొడక్ట్ పేరు గుర్తించలేకపోయాను. దయచేసి ప్రొడక్ట్ పేరును కూడా చెప్పండి."
			break
		}
		updated, sale, err := s.store.RecordSale(product.ID, intent.Quantity, time.Now())
		if err != nil {
			response.Success = false
			english, telugu = friendlyStoreError(err, product)
			break
		}
		response.Changed = true
		english = fmt.Sprintf("Sale recorded: %d %s of %s for %s. Remaining stock is %d.", intent.Quantity, product.Unit, product.Name, formatMoney(sale.Total), updated.Stock)
		telugu = fmt.Sprintf("అమ్మకం నమోదు చేశాను: %s %d %s, మొత్తం %s. మిగిలిన స్టాక్ %d.", product.Name, intent.Quantity, product.Unit, formatMoney(sale.Total), updated.Stock)

	case intents.Restock:
		product, ok := productByID(products, intent.ProductID)
		if !ok {
			response.Success = false
			english = "I heard a restock command, but I could not identify the product. Please include the product name."
			telugu = "స్టాక్ చేర్చే కమాండ్ అర్థమైంది, కానీ ప్రొడక్ట్ పేరు గుర్తించలేకపోయాను. దయచేసి ప్రొడక్ట్ పేరును చెప్పండి."
			break
		}
		updated, err := s.store.Restock(product.ID, intent.Quantity, "Voice restock", time.Now())
		if err != nil {
			response.Success = false
			english, telugu = friendlyStoreError(err, product)
			break
		}
		response.Changed = true
		english = fmt.Sprintf("Added %d %s of %s. New stock is %d.", intent.Quantity, product.Unit, product.Name, updated.Stock)
		telugu = fmt.Sprintf("%s కు %d %s స్టాక్ చేర్చాను. కొత్త స్టాక్ %d.", product.Name, intent.Quantity, product.Unit, updated.Stock)

	case intents.LowStock:
		dashboard := s.store.Dashboard(time.Now())
		low := make([]string, 0)
		for _, product := range dashboard.Products {
			if product.Stock <= product.LowStockThreshold {
				low = append(low, fmt.Sprintf("%s (%d)", product.Name, product.Stock))
			}
		}
		if len(low) == 0 {
			english = "No product is below its safe-stock level."
			telugu = "ఏ ప్రొడక్ట్ కూడా సేఫ్ స్టాక్ స్థాయి కంటే తక్కువగా లేదు."
		} else {
			english = "Low-stock products: " + strings.Join(low, ", ") + "."
			telugu = "తక్కువ స్టాక్ ఉన్న ప్రొడక్ట్స్: " + strings.Join(low, ", ") + "."
		}

	case intents.SalesSummary:
		metrics := s.store.Dashboard(time.Now()).Metrics
		english = fmt.Sprintf("Today's sales are %s from %d orders and %d items. Estimated gross profit is %s.", formatMoney(metrics.TodayRevenue), metrics.TodayOrders, metrics.TodayItemsSold, formatMoney(metrics.TodayProfit))
		telugu = fmt.Sprintf("ఈ రోజు అమ్మకాలు %s, %d ఆర్డర్లు, %d ఐటమ్స్. అంచనా గ్రాస్ లాభం %s.", formatMoney(metrics.TodayRevenue), metrics.TodayOrders, metrics.TodayItemsSold, formatMoney(metrics.TodayProfit))

	case intents.ProfitSummary:
		metrics := s.store.Dashboard(time.Now()).Metrics
		margin := 0.0
		if metrics.TodayRevenue > 0 {
			margin = metrics.TodayProfit / metrics.TodayRevenue * 100
		}
		english = fmt.Sprintf("Today's estimated gross profit is %s with a %.1f percent margin.", formatMoney(metrics.TodayProfit), margin)
		telugu = fmt.Sprintf("ఈ రోజు అంచనా గ్రాస్ లాభం %s. మార్జిన్ %.1f శాతం.", formatMoney(metrics.TodayProfit), margin)

	case intents.ReorderPlan:
		dashboard := s.store.Dashboard(time.Now())
		english, telugu = reorderReply(dashboard.ReorderPlan)

	case intents.TopProducts:
		dashboard := s.store.Dashboard(time.Now())
		if len(dashboard.TopProducts) == 0 {
			english = "There are no sales yet to rank products."
			telugu = "ప్రొడక్ట్స్ ర్యాంక్ చేయడానికి ఇంకా అమ్మకాలు లేవు."
		} else {
			limit := minInt(3, len(dashboard.TopProducts))
			parts := make([]string, 0, limit)
			for _, item := range dashboard.TopProducts[:limit] {
				parts = append(parts, fmt.Sprintf("%s %d", item.ProductName, item.Quantity))
			}
			english = "Top-selling products in the last seven days: " + strings.Join(parts, ", ") + "."
			telugu = "గత ఏడు రోజుల్లో ఎక్కువగా అమ్ముడైన ప్రొడక్ట్స్: " + strings.Join(parts, ", ") + "."
		}

	case intents.Briefing:
		dashboard := s.store.Dashboard(time.Now())
		top := "none yet"
		if len(dashboard.TopProducts) > 0 {
			top = dashboard.TopProducts[0].ProductName
		}
		english = fmt.Sprintf("Business briefing: today's revenue is %s, estimated profit is %s, %d products need attention, and the top seller this week is %s.", formatMoney(dashboard.Metrics.TodayRevenue), formatMoney(dashboard.Metrics.TodayProfit), len(dashboard.ReorderPlan), top)
		telugu = fmt.Sprintf("బిజినెస్ బ్రీఫింగ్: ఈ రోజు ఆదాయం %s, అంచనా లాభం %s, %d ప్రొడక్ట్స్‌కు శ్రద్ధ అవసరం, ఈ వారం టాప్ సెల్లర్ %s.", formatMoney(dashboard.Metrics.TodayRevenue), formatMoney(dashboard.Metrics.TodayProfit), len(dashboard.ReorderPlan), top)

	case intents.Undo:
		message, err := s.store.UndoLast(time.Now())
		if err != nil {
			response.Success = false
			english = "There is no recent sale or restock action that can be undone."
			telugu = "వెనక్కి తీసేయడానికి ఇటీవలి అమ్మకం లేదా రీస్టాక్ చర్య లేదు."
		} else {
			response.Changed = true
			english = message + "."
			telugu = "చివరి చర్యను వెనక్కి తీసేశాను."
		}

	default:
		response.Success = false
		english = "I could not confidently understand that command. Try asking about stock, recording a sale, today's sales, low stock or the reorder plan."
		telugu = "ఆ కమాండ్‌ను స్పష్టంగా అర్థం చేసుకోలేకపోయాను. స్టాక్, అమ్మకం నమోదు, ఈ రోజు అమ్మకాలు, తక్కువ స్టాక్ లేదా రీఆర్డర్ ప్లాన్ గురించి అడగండి."
	}

	response.Message = choose(language, english, telugu)
	response.SpeechText = response.Message
	response.Dashboard = s.store.Dashboard(time.Now())
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSTT(w http.ResponseWriter, r *http.Request) {
	if !s.gnani.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GNANI_NOT_CONFIGURED", "Add GNANI_API_KEY to the .env file, then restart the server", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes)
	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_AUDIO_UPLOAD", "Could not read the audio upload", err.Error())
		return
	}
	file, header, err := firstAudioFile(r.MultipartForm)
	if err != nil {
		writeError(w, http.StatusBadRequest, "AUDIO_REQUIRED", "Record an audio command first", err.Error())
		return
	}
	defer file.Close()

	audio, err := io.ReadAll(io.LimitReader(file, s.cfg.MaxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "AUDIO_READ_FAILED", "Could not read the recorded audio", err.Error())
		return
	}
	if int64(len(audio)) > s.cfg.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "AUDIO_TOO_LARGE", "Keep voice commands below 25 seconds", nil)
		return
	}

	products := s.store.Products()
	keyterms := make([]string, 0, len(products)*2)
	for _, product := range products {
		keyterms = append(keyterms, product.Name)
		if len(product.Aliases) > 0 {
			keyterms = append(keyterms, product.Aliases[0])
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.cfg.ExternalTimeoutSec)*time.Second)
	defer cancel()
	result, err := s.gnani.Transcribe(ctx, header.Filename, audio, gnani.STTOptions{
		Model:    strings.TrimSpace(r.FormValue("model")),
		Language: strings.TrimSpace(r.FormValue("language")),
		Format:   strings.TrimSpace(r.FormValue("format")),
		Topic:    "A small Indian retail shop owner checking stock, recording sales and requesting business summaries",
		Keyterms: keyterms,
	})
	if err != nil {
		handleGnaniError(w, err, "Speech recognition failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "result": result})
}

func (s *Server) handleTTS(w http.ResponseWriter, r *http.Request) {
	var request ttsRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Text) == "" {
		writeError(w, http.StatusBadRequest, "TEXT_REQUIRED", "Text-to-speech needs response text", nil)
		return
	}
	if request.Language == "" || request.Language == "auto" {
		request.Language = intents.DetectLanguage(request.Text)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.cfg.ExternalTimeoutSec)*time.Second)
	defer cancel()
	result, err := s.gnani.Synthesize(ctx, request.Text, gnani.TTSOptions{
		Model: request.Model, Language: request.Language, Voice: request.Voice, Speed: request.Speed,
	})
	if err != nil {
		handleGnaniError(w, err, "Speech synthesis failed")
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", `inline; filename="vyapari-response.wav"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Gnani-Model", result.Model)
	w.Header().Set("X-Gnani-Voice", result.Voice)
	w.Header().Set("X-Gnani-Language", result.Language)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Audio)
}

func (s *Server) handleAddProduct(w http.ResponseWriter, r *http.Request) {
	var request productRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	product, err := s.store.AddProduct(domain.Product{
		Name: request.Name, Aliases: request.Aliases, Category: request.Category, Unit: request.Unit,
		Stock: request.Stock, LowStockThreshold: request.LowStockThreshold, TargetStock: request.TargetStock,
		CostPrice: request.CostPrice, SellPrice: request.SellPrice,
	}, time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, "PRODUCT_INVALID", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true, "product": product, "dashboard": s.store.Dashboard(time.Now()),
	})
}

func (s *Server) handleStockAction(w http.ResponseWriter, r *http.Request) {
	var request stockActionRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_QUANTITY", "Quantity must be greater than zero", nil)
		return
	}

	var err error
	switch strings.ToLower(strings.TrimSpace(request.Action)) {
	case "sale":
		_, _, err = s.store.RecordSale(request.ProductID, request.Quantity, time.Now())
	case "restock":
		_, err = s.store.Restock(request.ProductID, request.Quantity, request.Note, time.Now())
	default:
		writeError(w, http.StatusBadRequest, "INVALID_ACTION", "Action must be sale or restock", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "STOCK_ACTION_FAILED", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "dashboard": s.store.Dashboard(time.Now())})
}

func (s *Server) handleUndo(w http.ResponseWriter, r *http.Request) {
	message, err := s.store.UndoLast(time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, "NOTHING_TO_UNDO", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": message, "dashboard": s.store.Dashboard(time.Now())})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Reset(time.Now()); err != nil {
		writeError(w, http.StatusInternalServerError, "RESET_FAILED", "Could not reset demo data", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "dashboard": s.store.Dashboard(time.Now())})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="vyapari-inventory.csv"`)
	if err := s.store.ExportCSV(w); err != nil {
		logCSVError(err)
	}
}

func (s *Server) handleReorderExport(w http.ResponseWriter, r *http.Request) {
	dashboard := s.store.Dashboard(time.Now())
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="vyapari-reorder-plan.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"Product", "Current stock", "Recommended quantity", "Priority", "Average daily sales", "Days of cover", "Estimated cost", "Reason"})
	for _, item := range dashboard.ReorderPlan {
		days := fmt.Sprintf("%.1f", item.DaysOfCover)
		if item.DaysOfCover >= 999 {
			days = "No recent demand"
		}
		_ = writer.Write([]string{
			item.ProductName,
			strconv.Itoa(item.CurrentStock),
			strconv.Itoa(item.RecommendedQty),
			item.Priority,
			fmt.Sprintf("%.1f", item.AverageDaily),
			days,
			fmt.Sprintf("%.2f", item.EstimatedCost),
			item.Reason,
		})
	}
	writer.Flush()
}

func firstAudioFile(form *multipart.Form) (multipart.File, *multipart.FileHeader, error) {
	if form == nil {
		return nil, nil, errors.New("missing multipart form")
	}
	for _, key := range []string{"audio", "audio_file", "file"} {
		headers := form.File[key]
		if len(headers) == 0 {
			continue
		}
		file, err := headers[0].Open()
		return file, headers[0], err
	}
	return nil, nil, errors.New("missing audio field")
}

func handleGnaniError(w http.ResponseWriter, err error, fallbackMessage string) {
	if errors.Is(err, gnani.ErrNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "GNANI_NOT_CONFIGURED", "Add GNANI_API_KEY to .env and restart the server", nil)
		return
	}
	var apiErr *gnani.APIError
	if errors.As(err, &apiErr) {
		status := http.StatusBadGateway
		switch apiErr.StatusCode {
		case 400, 422:
			status = http.StatusBadRequest
		case 401, 403:
			status = http.StatusUnauthorized
		case 402:
			status = http.StatusPaymentRequired
		case 429:
			status = http.StatusTooManyRequests
		case 500, 502, 503, 504:
			status = http.StatusBadGateway
		}
		writeError(w, status, "GNANI_API_ERROR", fallbackMessage, apiErr.Error())
		return
	}
	writeError(w, http.StatusBadGateway, "GNANI_REQUEST_FAILED", fallbackMessage, err.Error())
}

func responseLanguage(requested, detected string) string {
	requested = strings.TrimSpace(requested)
	if requested == "" || requested == "auto" {
		requested = detected
	}
	switch requested {
	case "te", "te-IN":
		return "te-IN"
	case "hi", "hi-IN", "hi-en":
		return requested
	default:
		return "en-IN"
	}
}

func choose(language, english, telugu string) string {
	if language == "te-IN" {
		return telugu
	}
	return english
}

func defaultSuggestions(language string) []string {
	if language == "te-IN" {
		return []string{
			"Basmati rice stock entha undi?",
			"Basmati rice rendu packets sale ayindi",
			"Low stock products cheppu",
			"Ee roju sales entha?",
		}
	}
	return []string{
		"How much Basmati Rice stock is available?",
		"Sell two packets of Basmati Rice",
		"Show low-stock products",
		"Give me today's sales summary",
	}
}

func reorderReply(items []domain.ReorderItem) (string, string) {
	if len(items) == 0 {
		return "No reorder is currently recommended.", "ప్రస్తుతం రీఆర్డర్ అవసరం లేదు."
	}
	limit := minInt(4, len(items))
	englishParts := make([]string, 0, limit)
	teluguParts := make([]string, 0, limit)
	total := 0.0
	for _, item := range items[:limit] {
		englishParts = append(englishParts, fmt.Sprintf("%s %d units", item.ProductName, item.RecommendedQty))
		teluguParts = append(teluguParts, fmt.Sprintf("%s %d యూనిట్స్", item.ProductName, item.RecommendedQty))
		total += item.EstimatedCost
	}
	return fmt.Sprintf("Recommended reorder: %s. Estimated purchase cost is %s.", strings.Join(englishParts, ", "), formatMoney(total)),
		fmt.Sprintf("రీఆర్డర్ సూచన: %s. అంచనా కొనుగోలు ఖర్చు %s.", strings.Join(teluguParts, ", "), formatMoney(total))
}

func friendlyStoreError(err error, product domain.Product) (string, string) {
	switch {
	case errors.Is(err, storepkg.ErrInsufficientStock):
		return fmt.Sprintf("Not enough %s is available. Current stock is %d.", product.Name, product.Stock),
			fmt.Sprintf("%s సరిపడా స్టాక్ లేదు. ప్రస్తుతం %d మాత్రమే ఉంది.", product.Name, product.Stock)
	case errors.Is(err, storepkg.ErrInvalidQuantity):
		return "Quantity must be greater than zero.", "క్వాంటిటీ సున్నా కంటే ఎక్కువగా ఉండాలి."
	default:
		return "The stock action could not be completed.", "స్టాక్ చర్యను పూర్తి చేయలేకపోయాను."
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func logCSVError(err error) {
	// The response may already have started, so logging is safer than writing JSON.
	fmt.Printf("CSV export failed: %v\n", err)
}

// Keep deterministic ordering when a response is derived from a map in future
// extensions. The helper is currently also useful in tests.
func sortedKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func finite(value float64) float64 {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return 0
	}
	return value
}
