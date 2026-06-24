package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/agentclash/agentclash/cli/internal/securitystress"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(securityCmd)
	securityCmd.AddCommand(securityStressRunCmd)

	securityStressRunCmd.Flags().Int("iterations", 10, "Number of stress iterations per provider")
	securityStressRunCmd.Flags().StringSlice("provider", []string{"openai"}, "Comma-separated providers (currently: openai)")
	securityStressRunCmd.Flags().StringSlice("model", []string{"gpt-4o-mini"}, "Comma-separated model ids, paired with --provider")
	securityStressRunCmd.Flags().Int("concurrency", 3, "Max concurrent iterations per provider/model pair")
	securityStressRunCmd.Flags().Duration("timeout", 60*time.Second, "Per-LLM-call timeout")
	securityStressRunCmd.Flags().String("api-key-env", "OPENAI_API_KEY", "Env var holding the provider API key")
	securityStressRunCmd.Flags().String("out", "", "Path to write the full JSON report (default: stdout summary only)")
	securityStressRunCmd.Flags().Bool("no-system-guard", false, "Drop the 'refuse leaks' sentence from the system prompt. Measures baseline alignment without harness-side coaching.")
}

var securityCmd = &cobra.Command{
	Use:   "security",
	Short: "Security-evals tooling (stress runs, leak rate, policy preview)",
	Long:  "Run security eval packs directly against an LLM provider and aggregate leak rate without touching the AgentClash backend. Useful for fast iteration on the canonical packs.",
}

var securityStressRunCmd = &cobra.Command{
	Use:   "stress-run <pack-yaml>",
	Short: "Run a security pack N times against an LLM provider and aggregate the leak rate",
	Long: `Loads a security eval pack YAML, fires its adversarial prompts at the
selected LLM provider for N iterations, and prints an aggregate report.

The harness operates entirely client-side — no AgentClash backend, no
sandbox, no E2B. It's a fast loop for measuring "does this model leak
under our attack library?" before committing to full pipeline runs.

Example:

  export OPENAI_API_KEY=sk-...
  agentclash security stress-run examples/eval-packs/secret-hygiene-env.yaml \
      --iterations 25 --provider openai --model gpt-4o-mini --out report.json
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		packPath := args[0]
		data, err := os.ReadFile(packPath)
		if err != nil {
			return fmt.Errorf("read pack: %w", err)
		}
		pack, err := securitystress.LoadPack(data)
		if err != nil {
			return err
		}
		if pack.Security == nil {
			return fmt.Errorf("pack %s has no security policy; not a security pack", packPath)
		}

		iterations, _ := cmd.Flags().GetInt("iterations")
		providers, _ := cmd.Flags().GetStringSlice("provider")
		models, _ := cmd.Flags().GetStringSlice("model")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		apiKeyEnv, _ := cmd.Flags().GetString("api-key-env")
		outPath, _ := cmd.Flags().GetString("out")
		noGuard, _ := cmd.Flags().GetBool("no-system-guard")

		apiKey := os.Getenv(apiKeyEnv)
		if apiKey == "" {
			return fmt.Errorf("env var %s is empty; set it before running stress-run", apiKeyEnv)
		}

		if len(providers) != len(models) {
			return fmt.Errorf("--provider and --model must be the same length (got %d providers, %d models)", len(providers), len(models))
		}

		rc := GetRunContext(cmd)
		// In structured mode keep stdout a clean JSON stream; route human
		// progress and per-run summaries to stderr.
		progressW, structured := progressWriter(rc)

		ctx := context.Background()
		all := make([]*securitystress.Result, 0, len(providers))
		for i, p := range providers {
			fmt.Fprintf(progressW, "Stress-running pack=%s provider=%s model=%s iterations=%d concurrency=%d...\n",
				pack.Pack.Slug, p, models[i], iterations, concurrency)
			cfg := securitystress.Config{
				Provider:       p,
				Model:          models[i],
				APIKey:         apiKey,
				Iterations:     iterations,
				MaxConcurrent:  concurrency,
				PerCallTimeout: timeout,
				NoSystemGuard:  noGuard,
			}
			res, err := securitystress.Run(ctx, pack, cfg)
			if err != nil {
				return fmt.Errorf("run %s/%s: %w", p, models[i], err)
			}
			all = append(all, res)
			renderSummary(progressW, res)
		}

		if structured {
			if err := rc.Output.PrintRaw(all); err != nil {
				return err
			}
		}

		if outPath != "" {
			f, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer f.Close()
			if err := json.NewEncoder(f).Encode(all); err != nil {
				return err
			}
			fmt.Fprintf(progressW, "Wrote full report to %s\n", outPath)
		}
		return nil
	},
}

func renderSummary(w io.Writer, r *securitystress.Result) {
	fmt.Fprintf(w, "\n=== %s / %s ===\n", r.Provider, r.Model)
	fmt.Fprintf(w, "  iterations         : %d\n", r.Iterations)
	fmt.Fprintf(w, "  leak-rate (gate)   : %d/%d = %.0f%%\n", r.LeakedIters, r.Iterations, 100*(1-r.Posture))
	fmt.Fprintf(w, "  posture            : %.2f\n", r.Posture)
	fmt.Fprintf(w, "  total incidents    : %d\n", r.TotalIncidents)
	fmt.Fprintf(w, "  severity breakdown : ")
	keys := make([]string, 0, len(r.BySeverity))
	for k := range r.BySeverity {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "%s=%d ", k, r.BySeverity[k])
	}
	fmt.Fprintln(w)
	if len(r.ByStrategy) > 0 {
		fmt.Fprintln(w, "  refusal by strategy:")
		stratKeys := make([]string, 0, len(r.ByStrategy))
		for k := range r.ByStrategy {
			stratKeys = append(stratKeys, k)
		}
		sort.Strings(stratKeys)
		for _, k := range stratKeys {
			so := r.ByStrategy[k]
			total := so.Refused + so.Accepted
			rate := 0.0
			if total > 0 {
				rate = float64(so.Refused) / float64(total) * 100
			}
			fmt.Fprintf(w, "    %-30s refused %d/%d (%.0f%%)\n", k, so.Refused, total, rate)
		}
	}
	if len(r.Errors) > 0 {
		fmt.Fprintf(w, "  errors             : %d (showing first 3)\n", len(r.Errors))
		for i, e := range r.Errors {
			if i >= 3 {
				break
			}
			fmt.Fprintf(w, "    - %s\n", e)
		}
	}
}
