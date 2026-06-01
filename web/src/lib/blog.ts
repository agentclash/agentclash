import fs from "fs";
import path from "path";
import matter from "gray-matter";

const CONTENT_DIR = path.join(process.cwd(), "content", "blog");

export type BlogPost = {
  slug: string;
  title: string;
  date: string;
  description: string;
  author: string;
};

export type BlogPostWithContent = BlogPost & {
  content: string;
};

function requiredText(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

export function parseBlogPost(slug: string, raw: string): BlogPostWithContent | null {
  try {
    const { data, content } = matter(raw);
    const title = requiredText(data.title);
    const date = requiredText(data.date);
    const description = requiredText(data.description);
    const author = requiredText(data.author);

    if (!title || !date || !description || !author) {
      return null;
    }

    return {
      slug,
      title,
      date,
      description,
      author,
      content,
    };
  } catch {
    return null;
  }
}

function readPostBySlug(slug: string): BlogPostWithContent | null {
  const filePath = path.join(CONTENT_DIR, `${slug}.mdx`);

  if (!fs.existsSync(filePath)) return null;

  try {
    return parseBlogPost(slug, fs.readFileSync(filePath, "utf-8"));
  } catch {
    return null;
  }
}

export function getAllPosts(): BlogPost[] {
  if (!fs.existsSync(CONTENT_DIR)) return [];
  const files = fs.readdirSync(CONTENT_DIR).filter((f) => f.endsWith(".mdx"));
  const posts: BlogPost[] = [];

  for (const filename of files) {
    const post = readPostBySlug(filename.replace(/\.mdx$/, ""));

    if (!post) continue;

    posts.push({
      slug: post.slug,
      title: post.title,
      date: post.date,
      description: post.description,
      author: post.author,
    });
  }

  return posts.sort((a, b) => (a.date > b.date ? -1 : 1));
}

export function getPostBySlug(slug: string): BlogPostWithContent | null {
  return readPostBySlug(slug);
}

export function getAllSlugs(): string[] {
  return getAllPosts().map((post) => post.slug);
}
