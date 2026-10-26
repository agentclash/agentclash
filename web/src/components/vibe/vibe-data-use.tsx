export function VibeDataUseNotice() {
  return <p className="vibe-material-hint">AgentClash stores your text and files; running sends selected content to configured model providers. Guest uploads expire after seven days unless claimed by signing in before expiry. <a className="underline underline-offset-4" href="/docs/vibe-evals/data-use" target="_blank" rel="noopener noreferrer">Data use and retention (new tab)</a></p>;
}

export function VibeRedactionHint() {
  return <p className="vibe-material-hint">Remove API keys and customer identifiers before pasting. For example: “Customer [NAME], email [EMAIL], API key [REDACTED].”</p>;
}
