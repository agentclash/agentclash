export function canRenderScorecardEmbed(resource: Record<string, unknown>) {
  return (
    resource.type === "run_scorecard" ||
    resource.type === "run_agent_scorecard"
  );
}
