package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config contains all runtime configuration. Values can be supplied through
// environment variables or a local .env file. Existing environment variables
// always take precedence over values in .env.
type Config struct {
	Addr               string
	DataFile           string
	GnaniAPIKey        string
	GnaniSTTURL        string
	GnaniTTSURL        string
	STTModel           string
	STTLanguage        string
	STTFormat          string
	STTFallbackModel   string
	STTFallbackLang    string
	TTSModel           string
	TTSSpeed           float64
	TTSVoiceTelugu     string
	TTSVoiceEnglish    string
	TTSVoiceHinglish   string
	TTSVoiceHindi      string
	MaxUploadBytes     int64
	ExternalTimeoutSec int
}

func Load() Config {
	loadDotEnv(".env")

	return Config{
		Addr:               env("APP_ADDR", ":8080"),
		DataFile:           env("DATA_FILE", "data/store.json"),
		GnaniAPIKey:        strings.TrimSpace(os.Getenv("GNANI_API_KEY")),
		GnaniSTTURL:        env("GNANI_STT_URL", "https://api.vachana.ai/stt/v3"),
		GnaniTTSURL:        env("GNANI_TTS_URL", "https://api.vachana.ai/api/v1/tts/inference"),
		STTModel:           env("GNANI_STT_MODEL", "prisma-v3.0"),
		STTLanguage:        env("GNANI_STT_LANGUAGE", "indic-auto"),
		STTFormat:          env("GNANI_STT_FORMAT", "codemix"),
		STTFallbackModel:   env("GNANI_STT_FALLBACK_MODEL", "prisma-v2.5"),
		STTFallbackLang:    env("GNANI_STT_FALLBACK_LANGUAGE", "te-IN"),
		TTSModel:           env("GNANI_TTS_MODEL", "timbre-v2.5"),
		TTSSpeed:           envFloat("GNANI_TTS_SPEED", 1.0),
		TTSVoiceTelugu:     env("GNANI_TTS_VOICE_TE", "Suhana"),
		TTSVoiceEnglish:    env("GNANI_TTS_VOICE_EN", "Kaveri"),
		TTSVoiceHinglish:   env("GNANI_TTS_VOICE_HINGLISH", "Poorvi"),
		TTSVoiceHindi:      env("GNANI_TTS_VOICE_HI", "Nalini"),
		MaxUploadBytes:     envInt64("MAX_AUDIO_UPLOAD_BYTES", 12<<20),
		ExternalTimeoutSec: envInt("EXTERNAL_TIMEOUT_SECONDS", 60),
	}
}

func (c Config) GnaniConfigured() bool {
	return strings.TrimSpace(c.GnaniAPIKey) != ""
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
