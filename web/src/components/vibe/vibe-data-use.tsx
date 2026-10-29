// A short notice next to every input. The data-use page holds the full storage, provider and expiry terms.
export function VibeDataUseNotice({ className = "" }: { className?: string }) {
  return <p className={`vibe-material-hint ${className}`.trim()}>Runs use model providers. Guest files expire in 7 days. Leave out secrets and customer IDs. <a className="underline underline-offset-4" href="/docs/vibe-evals/data-use" target="_blank" rel="noopener noreferrer">Data use<span className="sr-only"> and retention (new tab)</span></a></p>;
}

export function VibeRedactionHint() {
  return <p className="vibe-material-hint">Remove API keys and customer identifiers before pasting. For example: “Customer [NAME], email [EMAIL], API key [REDACTED].”</p>;
}
