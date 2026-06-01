import remarkGfm from "remark-gfm";
import type { MDXRemoteProps } from "next-mdx-remote/rsc";

/** Shared MDX compile options for docs, blog, and other RSC MDX surfaces. */
export const mdxRemoteOptions: MDXRemoteProps["options"] = {
  mdxOptions: {
    remarkPlugins: [remarkGfm],
  },
};
