import { getDocBySlug, renderDocMarkdown } from "@/lib/docs";

type Context = {
  params: Promise<{
    slug?: string[];
  }>;
};

export async function GET(_request: Request, context: Context) {
  const params = await context.params;
  const slug = params.slug ?? [];
  const doc = getDocBySlug(slug);

  if (!doc) {
    return new Response("Not found", {
      status: 404,
      headers: {
        "Content-Type": "text/plain; charset=utf-8",
      },
    });
  }

  return new Response(renderDocMarkdown(doc), {
    headers: {
      "Content-Type": "text/markdown; charset=utf-8",
    },
  });
}
