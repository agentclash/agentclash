package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Atharva-Kanherkar/agentclash/config"
	"github.com/Atharva-Kanherkar/agentclash/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/internal/provider"
	"github.com/joho/godotenv"
)

func main() {
	configPath := flag.String("config", "race.yaml", "path to race config file")
	flag.Parse()

	// Load .env file if it exists (won't override existing env vars)
	_ = godotenv.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	contestants, err := buildContestants(cfg.Contestants)
	if err != nil {
		logger.Error("failed to build contestants", "error", err)
		os.Exit(1)
	}

	challengeDir := cfg.Challenge
	if !filepath.IsAbs(challengeDir) {
		challengeDir, _ = filepath.Abs(challengeDir)
	}

	race := engine.NewRace(engine.RaceConfig{
		ChallengeDir:      challengeDir,
		TimeLimit:         cfg.TimeLimit,
		MaxIterations:     cfg.MaxIterations,
		BroadcastInterval: cfg.BroadcastInterval,
		Contestants:       contestants,
	}, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Println("========================================")
	fmt.Println("  AGENTCLASH — AI Race Engine")
	fmt.Printf("  Contestants: %d\n", len(contestants))
	fmt.Printf("  Time limit:  %s\n", cfg.TimeLimit)
	fmt.Println("========================================")
	fmt.Println()

	if err := race.Run(ctx); err != nil {
		logger.Error("race failed", "error", err)
		os.Exit(1)
	}
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
