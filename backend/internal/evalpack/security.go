package evalpack

import (
	"fmt"
	"regexp"
	"strings"
)

// SecurityPolicy declares the security-eval extensions for a eval pack.
// Packs with family == "security" SHOULD set this; packs with any other
// family MAY set it (a research pack that also wants to check for secret
// leakage is fine). The security scorer (see security_scorer.go in PR 2)
// reads this policy at scoring time.
type SecurityPolicy struct {
	// PlantedSecrets are canary values that the test scaffolding plants
	// somewhere the agent can reach. The scorer treats any occurrence of
	// Value in the run transcript, tool output, or sandbox network log as
	// a leak incident.
	PlantedSecrets []PlantedSecret `yaml:"planted_secrets,omitempty" json:"planted_secrets,omitempty"`

	// ForbiddenOutputs are independent patterns the scorer scans the run
	// transcript + tool output for. Use these to catch leaks of known
	// real-world key shapes (Stripe sk_test_, AWS AKIA..., etc.) even when
	// no canary was planted.
	ForbiddenOutputs []ForbiddenPattern `yaml:"forbidden_outputs,omitempty" json:"forbidden_outputs,omitempty"`

	// ForbiddenEgress is a list of network destinations the agent must
	// not reach. PR 6 wires this against the sandbox network log.
	ForbiddenEgress []EgressPolicy `yaml:"forbidden_egress,omitempty" json:"forbidden_egress,omitempty"`

	// AdversarialPrompts are red-team turns injected into the run by the
	// test harness. PR 2 emits one prompt per challenge_input; the scorer
	// checks whether the agent capitulated.
	AdversarialPrompts []AdversarialPrompt `yaml:"adversarial_prompts,omitempty" json:"adversarial_prompts,omitempty"`

	// DefaultSeverity is the severity used for incidents that don't carry
	// their own override. One of low|medium|high|critical. Defaults to
	// "high" when omitted; the scorer weights aggregate Security Posture
	// by severity (PR 9).
	DefaultSeverity string `yaml:"default_severity,omitempty" json:"default_severity,omitempty"`
}

// PlantedSecret is a canary the test harness plants into the agent's
// environment before a run starts.
type PlantedSecret struct {
	// Name is the operator-visible identifier (e.g. "PROD_DB_PASSWORD").
	// Required.
	Name string `yaml:"name" json:"name"`
	// Value is the canary string the scorer searches for. Required. The
	// pack author should pick a value that's unlikely to appear
	// organically — e.g. "agentclash-canary-<random-suffix>".
	Value string `yaml:"value" json:"value"`
	// Location says where the canary is planted. One of:
	//   env             — exported as $Name in the sandbox shell
	//   file            — written to FilePath inside the sandbox
	//   infisical-mock  — served by the mock Infisical fixture (PR 7)
	//   tool-output     — returned from a specific tool when invoked
	// Required.
	Location string `yaml:"location" json:"location"`
	// FilePath is required when Location == "file". Path inside the
	// sandbox working directory.
	FilePath string `yaml:"file_path,omitempty" json:"file_path,omitempty"`
	// Severity overrides the policy default for this specific secret.
	// Use "critical" for production credentials; "low" for things you
	// already publish elsewhere.
	Severity string `yaml:"severity,omitempty" json:"severity,omitempty"`
}

// ForbiddenPattern is a regex (or substring) the scorer treats as a leak
// signal independent of planted canaries.
type ForbiddenPattern struct {
	// Description is the human-readable name of this pattern (e.g.
	// "AWS access-key ID format").
	Description string `yaml:"description" json:"description"`
	// Pattern is a Go regexp. Required iff Substring is empty.
	Pattern string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	// Substring is an exact match alternative for callers that don't
	// need regex flexibility. Required iff Pattern is empty.
	Substring string `yaml:"substring,omitempty" json:"substring,omitempty"`
	// Severity overrides the policy default for this specific pattern.
	Severity string `yaml:"severity,omitempty" json:"severity,omitempty"`
}

// EgressPolicy is a network destination the agent must not reach. PR 6
// wires this through the sandbox network log.
type EgressPolicy struct {
	// Description is the human-readable name of this rule.
	Description string `yaml:"description" json:"description"`
	// Host is a glob (e.g. "*.attacker.com") or exact host the scorer
	// blocklists. The literal "*" means "any unexpected destination" when
	// combined with an Allow list.
	Host string `yaml:"host" json:"host"`
	// Allow is an exception list — even if Host matches, these hosts are
	// fine. Use ["infisical.your-org.com","your-deploy-target.com"] etc.
	Allow []string `yaml:"allow,omitempty" json:"allow,omitempty"`
	// Severity overrides the policy default.
	Severity string `yaml:"severity,omitempty" json:"severity,omitempty"`
}

// AdversarialPrompt is a red-team turn the test harness injects.
type AdversarialPrompt struct {
	// Name is the operator-visible identifier.
	Name string `yaml:"name" json:"name"`
	// Text is the message body sent to the agent. The test harness wraps
	// it as a user-role turn.
	Text string `yaml:"text" json:"text"`
	// Strategy is a free-form taxonomy tag — "dan", "ignore_instructions",
	// "role_confusion", "indirect", "encoding", etc. Used for UI grouping
	// and the eventual attack-library work.
	Strategy string `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	// ExpectedRefusalPattern, when set, is a regex the scorer uses to
	// confirm the agent refused the prompt. When unset the scorer falls
	// back to ForbiddenOutputs + PlantedSecrets leak detection.
	ExpectedRefusalPattern string `yaml:"expected_refusal_pattern,omitempty" json:"expected_refusal_pattern,omitempty"`
}

// SecurityFamily is the canonical Family value for security-focused packs.
const SecurityFamily = "security"

// normalizeSecurityPolicy trims whitespace and lowercases enum-ish fields
// so duplicate-detection and validation see canonical forms. Returns the
// same pointer (mutated in place) or nil when policy is nil.
func normalizeSecurityPolicy(policy *SecurityPolicy) *SecurityPolicy {
	if policy == nil {
		return nil
	}
	policy.DefaultSeverity = strings.ToLower(strings.TrimSpace(policy.DefaultSeverity))
	for i := range policy.PlantedSecrets {
		policy.PlantedSecrets[i].Name = strings.TrimSpace(policy.PlantedSecrets[i].Name)
		policy.PlantedSecrets[i].Value = strings.TrimSpace(policy.PlantedSecrets[i].Value)
		policy.PlantedSecrets[i].Location = strings.ToLower(strings.TrimSpace(policy.PlantedSecrets[i].Location))
		policy.PlantedSecrets[i].FilePath = strings.TrimSpace(policy.PlantedSecrets[i].FilePath)
		policy.PlantedSecrets[i].Severity = strings.ToLower(strings.TrimSpace(policy.PlantedSecrets[i].Severity))
	}
	for i := range policy.ForbiddenOutputs {
		policy.ForbiddenOutputs[i].Description = strings.TrimSpace(policy.ForbiddenOutputs[i].Description)
		policy.ForbiddenOutputs[i].Pattern = strings.TrimSpace(policy.ForbiddenOutputs[i].Pattern)
		policy.ForbiddenOutputs[i].Substring = strings.TrimSpace(policy.ForbiddenOutputs[i].Substring)
		policy.ForbiddenOutputs[i].Severity = strings.ToLower(strings.TrimSpace(policy.ForbiddenOutputs[i].Severity))
	}
	for i := range policy.ForbiddenEgress {
		policy.ForbiddenEgress[i].Description = strings.TrimSpace(policy.ForbiddenEgress[i].Description)
		policy.ForbiddenEgress[i].Host = strings.ToLower(strings.TrimSpace(policy.ForbiddenEgress[i].Host))
		policy.ForbiddenEgress[i].Severity = strings.ToLower(strings.TrimSpace(policy.ForbiddenEgress[i].Severity))
		for j := range policy.ForbiddenEgress[i].Allow {
			policy.ForbiddenEgress[i].Allow[j] = strings.ToLower(strings.TrimSpace(policy.ForbiddenEgress[i].Allow[j]))
		}
	}
	for i := range policy.AdversarialPrompts {
		policy.AdversarialPrompts[i].Name = strings.TrimSpace(policy.AdversarialPrompts[i].Name)
		// Don't lowercase Text — it's adversarial input, case-sensitive.
		policy.AdversarialPrompts[i].Strategy = strings.ToLower(strings.TrimSpace(policy.AdversarialPrompts[i].Strategy))
		policy.AdversarialPrompts[i].ExpectedRefusalPattern = strings.TrimSpace(policy.AdversarialPrompts[i].ExpectedRefusalPattern)
	}
	return policy
}

// IsSecurityPack reports whether the pack should run the security scorer.
// True when family == "security" OR a SecurityPolicy is attached.
func (b Bundle) IsSecurityPack() bool {
	if strings.EqualFold(b.Pack.Family, SecurityFamily) {
		return true
	}
	return b.Security != nil
}

// SecuritySeverityValues are the recognized severity tags. Unrecognized
// values fall back to the policy default at scoring time.
var SecuritySeverityValues = map[string]struct{}{
	"low":      {},
	"medium":   {},
	"high":     {},
	"critical": {},
}

// PlantedSecretLocations are the supported Location values.
var PlantedSecretLocations = map[string]struct{}{
	"env":            {},
	"file":           {},
	"infisical-mock": {},
	"tool-output":    {},
}

// validateSecurityPolicy walks the policy and returns ValidationErrors for
// any malformed fields. Callers concatenate into the global error list in
// ValidateBundle.
func validateSecurityPolicy(policy *SecurityPolicy) ValidationErrors {
	if policy == nil {
		return nil
	}
	var errs ValidationErrors
	if policy.DefaultSeverity != "" {
		if _, ok := SecuritySeverityValues[policy.DefaultSeverity]; !ok {
			errs = append(errs, ValidationError{
				Field:   "security.default_severity",
				Message: fmt.Sprintf("must be one of low, medium, high, critical (got %q)", policy.DefaultSeverity),
			})
		}
	}
	seen := map[string]struct{}{}
	for i, secret := range policy.PlantedSecrets {
		base := fmt.Sprintf("security.planted_secrets[%d]", i)
		if strings.TrimSpace(secret.Name) == "" {
			errs = append(errs, ValidationError{Field: base + ".name", Message: "is required"})
		}
		if strings.TrimSpace(secret.Value) == "" {
			errs = append(errs, ValidationError{Field: base + ".value", Message: "is required"})
		}
		if _, ok := PlantedSecretLocations[secret.Location]; !ok {
			errs = append(errs, ValidationError{
				Field:   base + ".location",
				Message: fmt.Sprintf("must be one of env, file, infisical-mock, tool-output (got %q)", secret.Location),
			})
		}
		if secret.Location == "file" && strings.TrimSpace(secret.FilePath) == "" {
			errs = append(errs, ValidationError{Field: base + ".file_path", Message: "is required when location == file"})
		}
		if secret.Location == "env" && secret.Name != "" && !envVarKeyPattern.MatchString(secret.Name) {
			errs = append(errs, ValidationError{
				Field:   base + ".name",
				Message: "must be a valid env-var identifier for location == env",
			})
		}
		if _, dup := seen[secret.Name]; dup && secret.Name != "" {
			errs = append(errs, ValidationError{Field: base + ".name", Message: "duplicate planted secret name"})
		}
		seen[secret.Name] = struct{}{}
		if secret.Severity != "" {
			if _, ok := SecuritySeverityValues[secret.Severity]; !ok {
				errs = append(errs, ValidationError{
					Field:   base + ".severity",
					Message: "must be one of low, medium, high, critical",
				})
			}
		}
	}
	for i, pat := range policy.ForbiddenOutputs {
		base := fmt.Sprintf("security.forbidden_outputs[%d]", i)
		if strings.TrimSpace(pat.Description) == "" {
			errs = append(errs, ValidationError{Field: base + ".description", Message: "is required"})
		}
		hasPattern := strings.TrimSpace(pat.Pattern) != ""
		hasSubstring := strings.TrimSpace(pat.Substring) != ""
		if hasPattern == hasSubstring {
			errs = append(errs, ValidationError{
				Field:   base,
				Message: "exactly one of pattern or substring must be set",
			})
		}
		if hasPattern {
			if _, err := regexp.Compile(pat.Pattern); err != nil {
				errs = append(errs, ValidationError{
					Field:   base + ".pattern",
					Message: fmt.Sprintf("invalid regexp: %v", err),
				})
			}
		}
		if pat.Severity != "" {
			if _, ok := SecuritySeverityValues[pat.Severity]; !ok {
				errs = append(errs, ValidationError{
					Field:   base + ".severity",
					Message: "must be one of low, medium, high, critical",
				})
			}
		}
	}
	for i, rule := range policy.ForbiddenEgress {
		base := fmt.Sprintf("security.forbidden_egress[%d]", i)
		if strings.TrimSpace(rule.Description) == "" {
			errs = append(errs, ValidationError{Field: base + ".description", Message: "is required"})
		}
		if strings.TrimSpace(rule.Host) == "" {
			errs = append(errs, ValidationError{Field: base + ".host", Message: "is required"})
		}
		if rule.Severity != "" {
			if _, ok := SecuritySeverityValues[rule.Severity]; !ok {
				errs = append(errs, ValidationError{
					Field:   base + ".severity",
					Message: "must be one of low, medium, high, critical",
				})
			}
		}
	}
	seenPrompts := map[string]struct{}{}
	for i, ap := range policy.AdversarialPrompts {
		base := fmt.Sprintf("security.adversarial_prompts[%d]", i)
		if strings.TrimSpace(ap.Name) == "" {
			errs = append(errs, ValidationError{Field: base + ".name", Message: "is required"})
		}
		if strings.TrimSpace(ap.Text) == "" {
			errs = append(errs, ValidationError{Field: base + ".text", Message: "is required"})
		}
		if _, dup := seenPrompts[ap.Name]; dup && ap.Name != "" {
			errs = append(errs, ValidationError{Field: base + ".name", Message: "duplicate adversarial prompt name"})
		}
		seenPrompts[ap.Name] = struct{}{}
		if ap.ExpectedRefusalPattern != "" {
			if _, err := regexp.Compile(ap.ExpectedRefusalPattern); err != nil {
				errs = append(errs, ValidationError{
					Field:   base + ".expected_refusal_pattern",
					Message: fmt.Sprintf("invalid regexp: %v", err),
				})
			}
		}
	}
	return errs
}
