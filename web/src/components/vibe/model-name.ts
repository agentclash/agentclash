type NamedModel = { id: string; name?: string };
const providers: Record<string, string> = { openai: "OpenAI", anthropic: "Anthropic", google: "Google", meta: "Meta", mistral: "Mistral" };
export function friendlyModelName(model: NamedModel, withProvider = false) {
  const [provider, ...parts] = model.id.split("/");
  const raw = parts.join("/") || provider;
  const name = model.name && model.name !== model.id && !model.name.includes("/") ? model.name : raw
    .replace(/^gpt-/i, "GPT-").replace(/-/g, " ").replace(/^GPT /, "GPT-")
    .replace(/^(claude|gemini|llama|mistral|qwen)\b/i, word => word[0].toUpperCase() + word.slice(1));
  const vendor = parts.length ? providers[provider] || provider[0].toUpperCase() + provider.slice(1) : "";
  return withProvider && vendor && !name.includes(`· ${vendor}`) ? `${name} · ${vendor}` : name;
}
