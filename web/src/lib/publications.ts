import type {
  PublicPublicationResponse,
  PublicShareResourceType,
} from "@/lib/api/types";
import { escapeMarkdownText } from "@/lib/agent-opportunity-markdown";
import { PUBLIC_ORIGIN } from "@/lib/public-content";

type UnknownRecord = Record<string, unknown>;

function record(value: unknown): UnknownRecord {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as UnknownRecord)
    : {};
}

function records(value: unknown): UnknownRecord[] {
  return Array.isArray(value) ? value.map(record) : [];
}

function text(value: unknown, fallback = "Not provided"): string {
  return typeof value === "string" && value.trim()
    ? escapeMarkdownText(value)
    : fallback;
}

function indentedJSON(value: unknown): string[] {
  const serialized = JSON.stringify(value ?? {}, null, 2);
  return serialized.split("\n").map((line) => `    ${line}`);
}

function score(value: unknown): string {
  return typeof value === "number" ? `${Math.round(value * 1000) / 10}%` : "N/A";
}

export function publicationTitle(publication: PublicPublicationResponse): string {
  const resource = publication.resource;
  if (resource.type === "challenge_pack_version") {
    const pack = record(resource.pack);
    const version = record(resource.version);
    return `${String(pack.name ?? "Challenge pack")} v${String(version.version_number ?? "")}`.trim();
  }
  if (resource.type === "run_scorecard") {
    return `${String(record(resource.run).name ?? "Run")} scorecard`;
  }
  if (resource.type === "run_agent_scorecard") {
    return `${String(record(resource.run_agent).label ?? "Agent")} scorecard`;
  }
  if (resource.type === "run_agent_replay") {
    return `${String(record(resource.run_agent).label ?? "Agent")} replay`;
  }
  if (resource.type === "agent_tryout") {
    return `${String(resource.template_slug ?? "Agent")} tryout`;
  }
  return "AgentClash publication";
}

export function publicationDescription(publication: PublicPublicationResponse): string {
  const labels: Record<PublicShareResourceType, string> = {
    report_snapshot: "Selected report",
    challenge_pack_version: "A redacted, user-published AgentClash challenge pack version.",
    run_scorecard: "A redacted, user-published AgentClash run scorecard.",
    run_agent_scorecard: "A redacted, user-published AgentClash agent scorecard.",
    run_agent_replay: "A redacted, user-published AgentClash agent replay.",
    agent_tryout: "A redacted, user-published AgentClash agent tryout.",
  };
  return labels[publication.resource.type];
}

export function renderPublicationCatalogMarkdown(
  publications: PublicPublicationResponse[],
  origin = PUBLIC_ORIGIN,
): string {
  const lines = [
    "# Published Agent Evaluation Artifacts",
    "",
    "Browse explicitly published, redacted AgentClash challenge packs, scorecards, replays, and agent tryouts.",
    "",
    `Source: ${origin}/publications`,
    `Markdown export: ${origin}/md/publications`,
    "",
    "Only active, unexpired artifacts whose owners enabled search indexing appear here. Capability-token shares are never listed.",
    "",
    "## Active publications",
    "",
  ];
  if (publications.length === 0) {
    lines.push("No active publications are currently available.");
  } else {
    for (const publication of publications) {
      const title = text(publicationTitle(publication));
      const path = publication.publication.canonical_path;
      lines.push(
        `- [${title}](${origin}/md${path}) — ${publicationDescription(publication)} Updated ${publication.publication.updated_at}.`,
      );
    }
  }
  return lines.join("\n").trim();
}

export function renderPublicationMarkdown(
  publication: PublicPublicationResponse,
  origin = PUBLIC_ORIGIN,
): string {
  const canonicalPath = publication.publication.canonical_path;
  const resource = publication.resource;
  const lines = [
    `# ${text(publicationTitle(publication))}`,
    "",
    "> User-published content. AgentClash renders this artifact through a redacted, allowlisted public serializer.",
    "",
    publicationDescription(publication),
    "",
    `Source: ${origin}${canonicalPath}`,
    `Markdown export: ${origin}/md${canonicalPath}`,
    `Published: ${publication.publication.created_at}`,
    `Updated: ${publication.publication.updated_at}`,
  ];

  if (resource.type === "challenge_pack_version") {
    const pack = record(resource.pack);
    const version = record(resource.version);
    lines.push(
      "",
      "## Challenge pack",
      "",
      `- Name: ${text(pack.name)}`,
      `- Family: ${text(pack.family)}`,
      `- Description: ${text(pack.description)}`,
      `- Version: ${String(version.version_number ?? "N/A")}`,
      `- Status: ${text(version.lifecycle_status)}`,
      "",
      "## Manifest",
      "",
      ...indentedJSON(version.manifest),
    );
    const inputSets = records(version.input_sets);
    if (inputSets.length > 0) {
      lines.push(
        "",
        "## Input sets",
        "",
        ...inputSets.map(
          (inputSet) => `- ${text(inputSet.name)} (${text(inputSet.input_key)})`,
        ),
      );
    }
  } else if (resource.type === "run_scorecard") {
    const run = record(resource.run);
    lines.push(
      "",
      "## Run",
      "",
      `- Name: ${text(run.name)}`,
      `- Status: ${text(run.status)}`,
      `- Execution mode: ${text(run.execution_mode)}`,
      "",
      "## Agent results",
      "",
      "| Agent | Status | Overall | Passed |",
      "| --- | --- | --- | --- |",
    );
    const scorecards = new Map(
      records(resource.agent_scorecards).map((item) => [String(item.run_agent_id), item]),
    );
    for (const agent of records(resource.agents)) {
      const card = scorecards.get(String(agent.id));
      lines.push(
        `| ${text(agent.label)} | ${text(agent.status)} | ${score(card?.overall_score)} | ${card?.passed === true ? "Yes" : card?.passed === false ? "No" : "N/A"} |`,
      );
    }
    lines.push(
      "",
      "## Redacted comparison detail",
      "",
      ...indentedJSON(resource.scorecard),
    );
  } else if (resource.type === "run_agent_scorecard") {
    const run = record(resource.run);
    const agent = record(resource.run_agent);
    const card = record(resource.scorecard);
    lines.push(
      "",
      "## Agent scorecard",
      "",
      `- Run: ${text(run.name)}`,
      `- Agent: ${text(agent.label)}`,
      `- Status: ${text(agent.status)}`,
      `- Overall: ${score(card.overall_score)}`,
      `- Correctness: ${score(card.correctness_score)}`,
      `- Reliability: ${score(card.reliability_score)}`,
      `- Latency: ${score(card.latency_score)}`,
      `- Cost: ${score(card.cost_score)}`,
      `- Passed: ${card.passed === true ? "Yes" : card.passed === false ? "No" : "N/A"}`,
      "",
      "## Redacted scorecard detail",
      "",
      ...indentedJSON(card.scorecard),
    );
  } else if (resource.type === "run_agent_replay") {
    const run = record(resource.run);
    const agent = record(resource.run_agent);
    const replay = record(resource.replay);
    lines.push(
      "",
      "## Replay",
      "",
      `- Run: ${text(run.name)}`,
      `- Agent: ${text(agent.label)}`,
      `- Status: ${text(agent.status)}`,
      `- Events: ${String(replay.event_count ?? "N/A")}`,
      "",
      "## Redacted replay summary",
      "",
      ...indentedJSON(replay.summary),
    );
  } else if (resource.type === "agent_tryout") {
    lines.push(
      "",
      "## Tryout",
      "",
      `- Template: ${text(resource.template_slug)}`,
      `- Status: ${text(resource.status)}`,
      `- Redaction status: ${text(resource.redaction_status)}`,
      `- Latency: ${String(resource.latency_ms ?? "N/A")} ms`,
      "",
      "## Redacted input",
      "",
      ...indentedJSON(resource.input_snapshot),
      "",
      "## Redacted result",
      "",
      ...indentedJSON(resource.summary),
    );
    const artifacts = records(resource.artifacts);
    if (artifacts.length > 0) {
      lines.push(
        "",
        "## Approved artifact descriptors",
        "",
        ...artifacts.map(
          (artifact) =>
            `- ${text(artifact.key)} — ${text(artifact.type ?? artifact.content_type)}`,
        ),
      );
    }
  }

  return lines.join("\n").trim();
}
