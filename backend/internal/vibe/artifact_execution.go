package vibe

import "slices"

// Artifact revisions keep target execution separate from the check blueprint.
// Copy slices so revising a target never mutates its historical baseline.
func inheritExecutionContext(dst, src *Artifact) {
	dst.AgentPrompt, dst.ScopeNote = src.AgentPrompt, src.ScopeNote
	dst.ReferenceInputs = slices.Clone(src.ReferenceInputs)
	dst.MissingReferences = slices.Clone(src.MissingReferences)
	dst.RequiredCapabilities = slices.Clone(src.RequiredCapabilities)
	if src.InputContract != nil {
		contract := *src.InputContract
		contract.Formats = slices.Clone(contract.Formats)
		dst.InputContract = &contract
	}
}

func bindExecutionContext(a *Artifact, p Plan) {
	if a.InputContract == nil {
		a.InputContract = materialContract()
	}
	if len(p.RequiredCapabilities) > 0 {
		a.RequiredCapabilities = slices.Clone(p.RequiredCapabilities)
	}
	if len(a.ReferenceInputs) == 0 {
		a.ReferenceInputs = referenceInputs(p)
	}
	if a.ScopeNote == "" {
		a.ScopeNote = prototypeScope + " It can process supplied text. PDF text is available only after a successful upload; images, linked resources and live tools are not read."
	}
}

// One execution-readiness rule is shared by admission and the worker.
func artifactReady(a Artifact, materials bool) error {
	if len(a.MissingReferences) > 0 {
		return fault("reference_required", "Attach the original reference materials before trying or checking this agent. Imported tests have been preserved; nothing was run.")
	}
	if a.UnavailableReason != "" {
		return fault("unsupported_capability", a.UnavailableReason)
	}
	if len(a.ReferenceInputs) > 0 && !materials {
		return fault("hosted_disabled", "Material-based execution is unavailable. Your definition and files are preserved.")
	}
	return nil
}
