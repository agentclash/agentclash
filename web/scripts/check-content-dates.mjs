import fs from "node:fs";
import path from "node:path";
import matter from "gray-matter";
import yaml from "js-yaml";

const roots = process.argv.slice(2);
if (!roots.length) roots.push("content/blog", "content/docs");
const today = new Date().toISOString().slice(0, 10);
let errors = 0;

function checkFile(file) {
  let data;
  try {
    data = matter(fs.readFileSync(file, "utf8"), {
      engines: { yaml: (source) => yaml.load(source, { schema: yaml.JSON_SCHEMA }) },
    }).data;
  } catch (error) {
    console.error(`${file}: invalid frontmatter: ${error.message}`);
    errors++;
    return;
  }
  const fields = ["date", "datePublished", "dateModified", "updated"];
  for (const field of fields) {
    if (!(field in data)) continue;
    const value = data[field];
    const parsed = typeof value === "string" && /^\d{4}-\d{2}-\d{2}$/.test(value)
      ? new Date(`${value}T00:00:00Z`)
      : new Date(NaN);
    if (Number.isNaN(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== value) {
      console.error(`${file}: ${field} must be a valid ISO calendar date (YYYY-MM-DD)`);
      errors++;
    } else if (value > today) {
      console.error(`${file}: ${field} cannot be in the future (${value})`);
      errors++;
    }
  }
  const published = data.date ?? data.datePublished;
  const modified = data.dateModified ?? data.updated;
  if (typeof published === "string" && typeof modified === "string" &&
      /^\d{4}-\d{2}-\d{2}$/.test(published) && /^\d{4}-\d{2}-\d{2}$/.test(modified) && modified < published) {
    console.error(`${file}: modification date precedes publication date`);
    errors++;
  }
}
function walk(root) {
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    const file = path.join(root, entry.name);
    if (entry.isDirectory()) walk(file);
    else if (/\.mdx?$/.test(entry.name)) checkFile(file);
  }
}
for (const root of roots) walk(root);
if (errors) process.exitCode = 1;
else console.log("Authored content dates are valid.");
