import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PublicScorecardEmbedPage, { generateMetadata } from "./page";
import { createApiClient } from "@/lib/api/client";

vi.mock("@/lib/api/client", () => ({
  createApiClient: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
}));

const mockCreateApiClient = vi.mocked(createApiClient);

describe("/share/[token]/embed", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("loads a public run scorecard share and renders embeddable HTML", async () => {
    const get = vi.fn().mockResolvedValue(publicRunScorecardShare());
    mockCreateApiClient.mockReturnValue({ get } as never);

    const page = await PublicScorecardEmbedPage({
      params: Promise.resolve({ token: "share token" }),
    });
    const html = renderToStaticMarkup(page);

    expect(get).toHaveBeenCalledWith("/public/shares/share%20token");
    expect(html).toContain('data-agentclash-embed="scorecard"');
    expect(html).toContain("Quarterly support run");
    expect(html).toContain("Careful agent");
  });

  it("keeps embed metadata unindexed", async () => {
    const get = vi.fn().mockResolvedValue(publicRunScorecardShare());
    mockCreateApiClient.mockReturnValue({ get } as never);

    const metadata = await generateMetadata({
      params: Promise.resolve({ token: "share-token" }),
    });

    expect(metadata.title).toBe("Quarterly support run embed");
    expect(metadata.robots).toEqual({ index: false, follow: false });
  });

  it("rejects non-scorecard public share resources", async () => {
    const get = vi.fn().mockResolvedValue({
      ...publicRunScorecardShare(),
      resource: { type: "run_agent_replay" },
    });
    mockCreateApiClient.mockReturnValue({ get } as never);

    await expect(
      PublicScorecardEmbedPage({
        params: Promise.resolve({ token: "replay-share" }),
      }),
    ).rejects.toThrow("NEXT_NOT_FOUND");
  });
});

function publicRunScorecardShare() {
  return {
    share: {
      id: "share-1",
      resource_type: "run_scorecard",
      resource_id: "run-1",
      search_indexing: false,
      view_count: 3,
      created_at: "2026-05-11T10:00:00Z",
      updated_at: "2026-05-11T10:00:00Z",
    },
    resource: {
      type: "run_scorecard",
      run: {
        id: "run-1",
        name: "Quarterly support run",
        status: "completed",
      },
      agents: [
        {
          id: "agent-1",
          label: "Fast agent",
          lane_index: 0,
          status: "completed",
        },
        {
          id: "agent-2",
          label: "Careful agent",
          lane_index: 1,
          status: "completed",
        },
      ],
      agent_scorecards: [
        {
          run_agent_id: "agent-2",
          overall_score: 0.92,
          passed: true,
        },
      ],
      scorecard: {
        winning_run_agent_id: "agent-2",
      },
    },
  };
}
