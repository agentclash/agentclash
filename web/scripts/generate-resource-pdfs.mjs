import fs from "node:fs/promises";
import path from "node:path";
import React from "react";
import {
  Document,
  Link,
  Page,
  StyleSheet,
  Text,
  View,
  renderToFile,
} from "@react-pdf/renderer";

const root = path.resolve(import.meta.dirname, "..");
const libraryPath = path.join(root, "content/resources/ai-agent-eval-library.json");
const outDir = path.join(root, "public/resources");

const palette = {
  ink: "#101114",
  muted: "#5d626b",
  line: "#d9dce2",
  panel: "#f5f6f8",
  accent: "#225cff",
  accentDark: "#163899",
  white: "#ffffff",
};

const styles = StyleSheet.create({
  page: {
    padding: 44,
    fontFamily: "Helvetica",
    color: palette.ink,
    backgroundColor: palette.white,
  },
  brand: {
    fontSize: 10,
    letterSpacing: 1.4,
    textTransform: "uppercase",
    color: palette.accentDark,
    marginBottom: 18,
  },
  title: {
    fontSize: 34,
    lineHeight: 1.08,
    fontFamily: "Helvetica-Bold",
    marginBottom: 12,
  },
  kicker: {
    fontSize: 11,
    letterSpacing: 1.1,
    textTransform: "uppercase",
    color: palette.accent,
    marginBottom: 12,
  },
  description: {
    fontSize: 13,
    lineHeight: 1.5,
    color: palette.muted,
    width: "84%",
  },
  metaRow: {
    flexDirection: "row",
    gap: 10,
    marginTop: 24,
    marginBottom: 26,
  },
  metaBox: {
    flexGrow: 1,
    flexBasis: 0,
    border: `1px solid ${palette.line}`,
    borderRadius: 6,
    padding: 12,
    backgroundColor: palette.panel,
  },
  metaLabel: {
    fontSize: 8,
    letterSpacing: 0.9,
    textTransform: "uppercase",
    color: palette.muted,
    marginBottom: 5,
  },
  metaValue: {
    fontSize: 10,
    lineHeight: 1.35,
    fontFamily: "Helvetica-Bold",
  },
  section: {
    borderTop: `1px solid ${palette.line}`,
    paddingTop: 17,
    marginTop: 4,
    marginBottom: 18,
  },
  sectionTitle: {
    fontSize: 16,
    fontFamily: "Helvetica-Bold",
    marginBottom: 10,
  },
  itemRow: {
    flexDirection: "row",
    gap: 9,
    marginBottom: 8,
  },
  check: {
    width: 16,
    height: 16,
    borderRadius: 8,
    backgroundColor: palette.accent,
    color: palette.white,
    fontSize: 10,
    textAlign: "center",
    paddingTop: 2,
    fontFamily: "Helvetica-Bold",
  },
  itemText: {
    flex: 1,
    fontSize: 11,
    lineHeight: 1.45,
    color: palette.ink,
  },
  promptPanel: {
    marginTop: 12,
    padding: 16,
    borderRadius: 7,
    backgroundColor: "#101114",
  },
  promptTitle: {
    color: palette.white,
    fontSize: 12,
    fontFamily: "Helvetica-Bold",
    marginBottom: 7,
  },
  promptText: {
    color: "#d8dbe2",
    fontSize: 10,
    lineHeight: 1.45,
  },
  footer: {
    position: "absolute",
    left: 44,
    right: 44,
    bottom: 26,
    flexDirection: "row",
    justifyContent: "space-between",
    borderTop: `1px solid ${palette.line}`,
    paddingTop: 10,
  },
  footerText: {
    fontSize: 8,
    color: palette.muted,
  },
});

function ResourceDocument({ resource }) {
  return React.createElement(
    Document,
    {
      title: `${resource.title} | AgentClash`,
      author: "AgentClash",
      subject: resource.description,
      keywords: "AI agent evaluation, release gates, agent benchmarking, AgentClash",
    },
    React.createElement(
      Page,
      { size: "LETTER", style: styles.page },
      React.createElement(Text, { style: styles.brand }, "AgentClash Resource Library"),
      React.createElement(Text, { style: styles.kicker }, resource.kicker),
      React.createElement(Text, { style: styles.title }, resource.title),
      React.createElement(Text, { style: styles.description }, resource.description),
      React.createElement(
        View,
        { style: styles.metaRow },
        React.createElement(
          View,
          { style: styles.metaBox },
          React.createElement(Text, { style: styles.metaLabel }, "Audience"),
          React.createElement(Text, { style: styles.metaValue }, resource.audience),
        ),
        React.createElement(
          View,
          { style: styles.metaBox },
          React.createElement(Text, { style: styles.metaLabel }, "Use in"),
          React.createElement(Text, { style: styles.metaValue }, resource.readTime),
        ),
        React.createElement(
          View,
          { style: styles.metaBox },
          React.createElement(Text, { style: styles.metaLabel }, "Best next step"),
          React.createElement(Text, { style: styles.metaValue }, "Turn answers into a eval pack and gate."),
        ),
      ),
      ...resource.sections.map((section) =>
        React.createElement(
          View,
          { key: section.title, style: styles.section, wrap: false },
          React.createElement(Text, { style: styles.sectionTitle }, section.title),
          ...section.items.map((item, index) =>
            React.createElement(
              View,
              { key: item, style: styles.itemRow },
              React.createElement(Text, { style: styles.check }, String(index + 1)),
              React.createElement(Text, { style: styles.itemText }, item),
            ),
          ),
        ),
      ),
      React.createElement(
        View,
        { style: styles.promptPanel },
        React.createElement(Text, { style: styles.promptTitle }, "How to use this in AgentClash"),
        React.createElement(
          Text,
          { style: styles.promptText },
          "Pick one workflow, freeze it as a eval pack, record a baseline run, compare the candidate under the same tools and budget, then promote critical misses into CI release gates.",
        ),
      ),
      React.createElement(
        View,
        { style: styles.footer, fixed: true },
        React.createElement(Text, { style: styles.footerText }, "agentclash.dev/resources/eval-checklist"),
        React.createElement(
          Link,
          { src: "https://www.agentclash.dev/enterprise", style: styles.footerText },
          "Book an eval architecture review",
        ),
      ),
    ),
  );
}

const raw = await fs.readFile(libraryPath, "utf8");
const library = JSON.parse(raw);
await fs.mkdir(outDir, { recursive: true });

for (const resource of library.resources) {
  const filename = `${resource.slug}.pdf`;
  await renderToFile(
    React.createElement(ResourceDocument, { resource }),
    path.join(outDir, filename),
  );
  console.log(`generated ${filename}`);
}
