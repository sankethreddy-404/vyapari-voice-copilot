package gnani

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vyapari-voice-copilot/internal/config"
)

func TestTranscribeSendsAPIKeyAndMultipartFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key-ID"); got != "secret" {
			t.Fatalf("api key header=%q", got)
		}
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("model"); got != "prisma-v3.0" {
			t.Fatalf("model=%q", got)
		}
		if got := r.FormValue("language_code"); got != "indic-auto" {
			t.Fatalf("language=%q", got)
		}
		file, _, err := r.FormFile("audio_file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != "audio" {
			t.Fatalf("audio=%q", data)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":                true,
			"request_id":             "req-1",
			"transcript":             "Basmati rice stock entha undi",
			"model":                  "prisma-v3.0",
			"detected_language_code": "te-IN",
			"processing_time":        0.2,
			"end_to_end_latency":     0.3,
		})
	}))
	defer server.Close()

	cfg := config.Load()
	cfg.GnaniAPIKey = "secret"
	cfg.GnaniSTTURL = server.URL
	cfg.ExternalTimeoutSec = 5
	client := NewWithHTTPClient(cfg, &http.Client{Timeout: 5 * time.Second})

	result, err := client.Transcribe(context.Background(), "voice.wav", []byte("audio"), STTOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Transcript == "" || result.Model != "prisma-v3.0" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSynthesizeReturnsAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key-ID") != "secret" {
			t.Fatal("missing api key")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["voice"] != "Suhana" {
			t.Fatalf("voice=%v", body["voice"])
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write([]byte("RIFFfakewav"))
	}))
	defer server.Close()

	cfg := config.Load()
	cfg.GnaniAPIKey = "secret"
	cfg.GnaniTTSURL = server.URL
	client := NewWithHTTPClient(cfg, server.Client())

	result, err := client.Synthesize(context.Background(), "నమస్తే", TTSOptions{Language: "te-IN"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(result.Audio), "RIFF") {
		t.Fatalf("audio=%q", result.Audio)
	}
}
