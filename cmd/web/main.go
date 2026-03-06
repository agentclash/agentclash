package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/Atharva-Kanherkar/agentclash/config"
	"github.com/Atharva-Kanherkar/agentclash/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/web"
	"github.com/joho/godotenv"
)

var (
	logger   *slog.Logger
	raceMu   sync.Mutex
	raceRunning bool
)

func main() {
	_ = godotenv.Load()
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	mux := http.NewServeMux()

	// Serve the UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := web.Static.ReadFile("index.html")
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

	// SSE endpoint for live events
	mux.HandleFunc("/api/events", handleSSE)

	// Start race endpoint
	mux.HandleFunc("/api/race/start", handleStartRace)

	addr := ":3000"
	logger.Info("AgentClash UI starting", "addr", "http://localhost"+addr)

	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()
	srv.Shutdown(context.Background())
}

var eventBus = engine.NewEventBus()

func handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := eventBus.Subscribe()
	defer eventBus.Unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func handleStartRace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raceMu.Lock()
	if raceRunning {
		raceMu.Unlock()
		http.Error(w, "race already running", http.StatusConflict)
		return
	}
	raceRunning = true
	raceMu.Unlock()

	cfg, err := config.Load("race.yaml")
	if err != nil {
		raceMu.Lock()
		raceRunning = false
		raceMu.Unlock()
		http.Error(w, "config error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	contestants, err := buildContestants(cfg.Contestants)
	if err != nil {
		raceMu.Lock()
		raceRunning = false
		raceMu.Unlock()
		http.Error(w, "contestant error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	challengeDir := cfg.Challenge
	if !filepath.IsAbs(challengeDir) {
		challengeDir, _ = filepath.Abs(challengeDir)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})

	go func() {
		defer func() {
			raceMu.Lock()
			raceRunning = false
			raceMu.Unlock()
		}()

		race := engine.NewRace(engine.RaceConfig{
			ChallengeDir:      challengeDir,
			TimeLimit:         cfg.TimeLimit,
			MaxIterations:     cfg.MaxIterations,
			BroadcastInterval: cfg.BroadcastInterval,
			Contestants:       contestants,
			Events:            eventBus,
		}, logger)

		if err := race.Run(context.Background()); err != nil {
			logger.Error("race failed", "error", err)
			eventBus.Emit(engine.Event{Type: "race_error", Data: map[string]any{"error": err.Error()}})
		}
	}()
}

func buildContestants(cfgs []config.ContestantConfig) ([]engine.Contestant, error) {
	var contestants []engine.Contestant
	for _, c := range cfgs {
		p, err := buildProvider(c.Provider)
		if err != nil {
			return nil, fmt.Errorf("provider for %s: %w", c.Name, err)
		}
		contestants = append(contestants, engine.Contestant{
			Name:     c.Name,
			Provider: p,
			Model:    c.Model,
		})
	}
	return contestants, nil
}

func buildProvider(name string) (provider.Provider, error) {
	switch name {
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not set")
		}
		return provider.NewOpenAI(provider.OpenAIConfig{APIKey: key}), nil
	case "anthropic":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
		return provider.NewAnthropic(provider.AnthropicConfig{APIKey: key}), nil
	case "gemini":
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY not set")
		}
		return provider.NewGemini(provider.GeminiConfig{APIKey: key}), nil
	case "openrouter":
		key := os.Getenv("OPENROUTER_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("OPENROUTER_API_KEY not set")
		}
		return provider.NewOpenAI(provider.OpenAIConfig{
			APIKey:  key,
			BaseURL: "https://openrouter.ai/api/v1",
		}), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}
