# Guided Build Authoring UX Notes

## Research Summary

The strongest OSS products converge on the same beginner-first pattern:

- Dify pushes new users toward templates first, not blank DSL. Its docs explicitly recommend creating applications from templates for beginners and only then dropping to blank or DSL import. Source: [Create Application - Dify Docs](https://docs.dify.ai/versions/3-0-x/en/user-guide/application-orchestrate/creating-an-application)
- Flowise splits the product into progressively more advanced layers. `Assistant` is described as the most beginner-friendly way to create an AI agent, while `Agentflow` is the superset for complex orchestration. Source: [Flowise Introduction](https://docs.flowiseai.com/)
- n8n’s simplest agent builder asks for a compact form: name, description, system prompt, preferred model, and tool access. More complex workflow agents live separately. Source: [n8n Chat Hub](https://docs.n8n.io/advanced-ai/chat-hub/)
- Langflow focuses agent setup around a single Agent component with model choice, tool connections, and instructions instead of asking the user to author a JSON bundle directly. Source: [Use Langflow agents](https://docs.langflow.org/agents)
- LibreChat exposes tools and OpenAPI actions through dedicated forms and menus rather than making users hand-author the whole internal representation. Source: [LibreChat Agents](https://www.librechat.ai/docs/features/agents)

## What This Means For AgentClash

For UI-first users, raw JSON spec editing is an advanced mode, not the primary authoring surface.

The product should therefore:

1. Start with templates and opinionated starters.
2. Expose the highest-value agent choices in plain language.
3. Keep raw JSON available as an expert escape hatch.
4. Preserve the same underlying product layer and API contracts so advanced and beginner users are editing the same artifact.

## UX Chosen In This PR

- `New Version` opens with starter templates instead of immediately creating a blank JSON draft.
- Build version editing defaults to a guided mode for role, mission, success criteria, input shape, tool strategy, memory style, and output contract.
- An `Advanced JSON` tab remains available and stays synchronized with the guided surface.
- The implementation writes back into the existing spec objects instead of introducing a second authoring system.
