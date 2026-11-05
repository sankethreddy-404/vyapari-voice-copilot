package gnani

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vyapari-voice-copilot/internal/config"
)

var ErrNotConfigured = errors.New("Gnani API key is not configured")

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	body := strings.TrimSpace(e.Body)
	if len(body) > 500 {
		body = body[:500] + "..."
	}
	return fmt.Sprintf("Gnani API returned HTTP %d: %s", e.StatusCode, body)
}

type Client struct {
	cfg        config.Config
	httpClient *http.Client
	semaphore  chan struct{}
	mu         sync.RWMutex
}

type STTOptions struct {
	Model    string
	Language string
	Format   string
	Topic    string
	Keyterms []string
}

type STTResult struct {
	Success              bool    `json:"success"`
	RequestID            string  `json:"requestId,omitempty"`
	Transcript           string  `json:"transcript"`
	Model                string  `json:"model,omitempty"`
	DetectedLanguageCode string  `json:"detectedLanguageCode,omitempty"`
	ProcessingTime       float64 `json:"processingTime,omitempty"`
	EndToEndLatency      float64 `json:"endToEndLatency,omitempty"`
	FallbackUsed         bool    `json:"fallbackUsed"`
}

type TTSOptions struct {
	Model    string  `json:"model,omitempty"`
	Language string  `json:"language,omitempty"`
	Voice    string  `json:"voice,omitempty"`
	Speed    float64 `json:"speed,omitempty"`
}

type TTSResult struct {
	Audio       []byte
	ContentType string
	Model       string
	Voice       string
	Language    string
}

func New(cfg config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.ExternalTimeoutSec) * time.Second,
		},
		// Programme accounts may allow only one active request. Serialising
		// calls avoids accidental 429 responses when users click rapidly.
		semaphore: make(chan struct{}, 1),
	}
}

func NewWithHTTPClient(cfg config.Config, client *http.Client) *Client {
	c := New(cfg)
	if client != nil {
		c.httpClient = client
	}
	return c
}

func (c *Client) Configured() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return strings.TrimSpace(c.cfg.GnaniAPIKey) != ""
}

func (c *Client) PublicConfig() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]any{
		"configured":  strings.TrimSpace(c.cfg.GnaniAPIKey) != "",
		"sttModel":    c.cfg.STTModel,
		"sttLanguage": c.cfg.STTLanguage,
		"sttFormat":   c.cfg.STTFormat,
		"ttsModel":    c.cfg.TTSModel,
	}
}

func (c *Client) Transcribe(ctx context.Context, filename string, audio []byte, options STTOptions) (STTResult, error) {
	if !c.Configured() {
		return STTResult{}, ErrNotConfigured
	}
	if len(audio) == 0 {
		return STTResult{}, errors.New("audio file is empty")
	}

	model := firstNonEmpty(options.Model, c.cfg.STTModel)
	language := firstNonEmpty(options.Language, c.cfg.STTLanguage)
	format := firstNonEmpty(options.Format, c.cfg.STTFormat)

	result, err := c.doSTT(ctx, filename, audio, STTOptions{
		Model: model, Language: language, Format: format, Topic: options.Topic, Keyterms: options.Keyterms,
	})
	if err == nil {
		return result, nil
	}

	var apiErr *APIError
	fallbackAllowed := strings.EqualFold(model, "prisma-v3.0") && errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusUnprocessableEntity)
	if !fallbackAllowed || strings.TrimSpace(c.cfg.STTFallbackModel) == "" {
		return STTResult{}, err
	}

	fallback, fallbackErr := c.doSTT(ctx, filename, audio, STTOptions{
		Model:    c.cfg.STTFallbackModel,
		Language: c.cfg.STTFallbackLang,
		Format:   "transcribe",
		Topic:    options.Topic,
		Keyterms: options.Keyterms,
	})
	if fallbackErr != nil {
		return STTResult{}, fmt.Errorf("primary STT failed (%v); fallback failed: %w", err, fallbackErr)
	}
	fallback.FallbackUsed = true
	return fallback, nil
}

func (c *Client) doSTT(ctx context.Context, filename string, audio []byte, options STTOptions) (STTResult, error) {
	release, err := c.acquire(ctx)
	if err != nil {
		return STTResult{}, err
	}
	defer release()

	var responseBody []byte
	var statusCode int
	for attempt := 0; attempt < 3; attempt++ {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, createErr := writer.CreateFormFile("audio_file", safeFilename(filename))
		if createErr != nil {
			return STTResult{}, fmt.Errorf("create multipart audio field: %w", createErr)
		}
		if _, writeErr := part.Write(audio); writeErr != nil {
			return STTResult{}, fmt.Errorf("write multipart audio: %w", writeErr)
		}
		if options.Model != "" {
			_ = writer.WriteField("model", options.Model)
		}
		_ = writer.WriteField("language_code", options.Language)
		if options.Format != "" {
			_ = writer.WriteField("format", options.Format)
		}
		if options.Topic != "" && strings.EqualFold(options.Model, "prisma-v3.0") {
			_ = writer.WriteField("topic", options.Topic)
		}
		if len(options.Keyterms) > 0 {
			encoded, _ := json.Marshal(options.Keyterms)
			if strings.EqualFold(options.Model, "prisma-v3.0") {
				_ = writer.WriteField("keyterms", string(encoded))
			} else {
				_ = writer.WriteField("bias_list", string(encoded))
				_ = writer.WriteField("bias_score", "1.5")
			}
		}
		if closeErr := writer.Close(); closeErr != nil {
			return STTResult{}, fmt.Errorf("finish multipart request: %w", closeErr)
		}

		req, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.GnaniSTTURL, body)
		if requestErr != nil {
			return STTResult{}, requestErr
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-API-Key-ID", c.cfg.GnaniAPIKey)
		req.Header.Set("Accept", "application/json")

		resp, requestErr := c.httpClient.Do(req)
		if requestErr != nil {
			if attempt < 2 && retryableNetworkError(requestErr) {
				if err := sleepContext(ctx, retryDelay(attempt)); err != nil {
					return STTResult{}, err
				}
				continue
			}
			return STTResult{}, fmt.Errorf("call Gnani STT: %w", requestErr)
		}
		responseBody, requestErr = io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		if requestErr != nil {
			return STTResult{}, fmt.Errorf("read Gnani STT response: %w", requestErr)
		}
		statusCode = resp.StatusCode
		if statusCode >= 200 && statusCode < 300 {
			break
		}
		if attempt < 2 && retryableStatus(statusCode) {
			if err := sleepContext(ctx, retryDelay(attempt)); err != nil {
				return STTResult{}, err
			}
			continue
		}
		return STTResult{}, &APIError{StatusCode: statusCode, Body: string(responseBody)}
	}

	var raw struct {
		Success              bool    `json:"success"`
		RequestID            string  `json:"request_id"`
		Transcript           string  `json:"transcript"`
		Model                string  `json:"model"`
		DetectedLanguageCode string  `json:"detected_language_code"`
		ProcessingTime       float64 `json:"processing_time"`
		EndToEndLatency      float64 `json:"end_to_end_latency"`
	}
	if err := json.Unmarshal(responseBody, &raw); err != nil {
		return STTResult{}, fmt.Errorf("decode Gnani STT response: %w", err)
	}
	return STTResult{
		Success:              raw.Success,
		RequestID:            raw.RequestID,
		Transcript:           strings.TrimSpace(raw.Transcript),
		Model:                raw.Model,
		DetectedLanguageCode: raw.DetectedLanguageCode,
		ProcessingTime:       raw.ProcessingTime,
		EndToEndLatency:      raw.EndToEndLatency,
	}, nil
}

func (c *Client) Synthesize(ctx context.Context, text string, options TTSOptions) (TTSResult, error) {
	if !c.Configured() {
		return TTSResult{}, ErrNotConfigured
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return TTSResult{}, errors.New("text is required")
	}
	if len([]rune(text)) > 1800 {
		return TTSResult{}, errors.New("text is too long for a single speech response")
	}

	language := normaliseTTSLanguage(firstNonEmpty(options.Language, "te-IN"))
	model := firstNonEmpty(options.Model, c.cfg.TTSModel)
	voice := firstNonEmpty(options.Voice, c.defaultVoice(language))
	speed := options.Speed
	if speed <= 0 {
		speed = c.cfg.TTSSpeed
	}

	release, err := c.acquire(ctx)
	if err != nil {
		return TTSResult{}, err
	}
	defer release()

	payload := map[string]any{
		"text":     text,
		"voice":    voice,
		"model":    model,
		"language": language,
		"speed":    speed,
		"audio_config": map[string]any{
			"sample_rate":  48000,
			"num_channels": 1,
			"sample_width": 2,
			"encoding":     "linear_pcm",
			"container":    "wav",
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return TTSResult{}, err
	}

	var responseBody []byte
	contentType := "audio/wav"
	for attempt := 0; attempt < 3; attempt++ {
		req, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.GnaniTTSURL, bytes.NewReader(encoded))
		if requestErr != nil {
			return TTSResult{}, requestErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key-ID", c.cfg.GnaniAPIKey)
		req.Header.Set("Accept", "audio/wav,application/octet-stream")

		resp, requestErr := c.httpClient.Do(req)
		if requestErr != nil {
			if attempt < 2 && retryableNetworkError(requestErr) {
				if err := sleepContext(ctx, retryDelay(attempt)); err != nil {
					return TTSResult{}, err
				}
				continue
			}
			return TTSResult{}, fmt.Errorf("call Gnani TTS: %w", requestErr)
		}
		responseBody, requestErr = io.ReadAll(io.LimitReader(resp.Body, 12<<20))
		_ = resp.Body.Close()
		if requestErr != nil {
			return TTSResult{}, fmt.Errorf("read Gnani TTS response: %w", requestErr)
		}
		if value := resp.Header.Get("Content-Type"); value != "" {
			contentType = value
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if len(responseBody) == 0 {
				return TTSResult{}, errors.New("Gnani TTS returned empty audio")
			}
			return TTSResult{Audio: responseBody, ContentType: contentType, Model: model, Voice: voice, Language: language}, nil
		}
		if attempt < 2 && retryableStatus(resp.StatusCode) {
			if err := sleepContext(ctx, retryDelay(attempt)); err != nil {
				return TTSResult{}, err
			}
			continue
		}
		return TTSResult{}, &APIError{StatusCode: resp.StatusCode, Body: string(responseBody)}
	}
	return TTSResult{}, errors.New("Gnani TTS request did not complete")
}

func (c *Client) defaultVoice(language string) string {
	switch language {
	case "en-IN":
		return c.cfg.TTSVoiceEnglish
	case "hi-IN":
		return c.cfg.TTSVoiceHindi
	case "hi-en":
		return c.cfg.TTSVoiceHinglish
	default:
		return c.cfg.TTSVoiceTelugu
	}
}

func (c *Client) acquire(ctx context.Context) (func(), error) {
	select {
	case c.semaphore <- struct{}{}:
		return func() { <-c.semaphore }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func safeFilename(filename string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "." || filename == "" {
		return "voice-command.wav"
	}
	return filename
}

func normaliseTTSLanguage(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "te", "te-IN":
		return "te-IN"
	case "en", "en-IN":
		return "en-IN"
	case "hi", "hi-IN":
		return "hi-IN"
	case "hi-en", "hinglish":
		return "hi-en"
	default:
		return "te-IN"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusInternalServerError || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func retryableNetworkError(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func retryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 450 * time.Millisecond
	}
	return time.Duration(attempt+1) * 850 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
