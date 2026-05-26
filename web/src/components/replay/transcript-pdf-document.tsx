import {
  Document,
  Page,
  View,
  Text,
  StyleSheet,
} from "@react-pdf/renderer";
import type { TranscriptTurn } from "@/lib/api/types";

export interface TranscriptPdfMeta {
  agentLabel: string;
  runName: string;
  runId: string;
  runAgentId: string;
  generatedAt: string;
}

// Print-friendly palette: dark text on white reads + prints far better than
// the app's dark instrument-panel theme. Student/agent accent hues mirror the
// on-screen transcript (cyan vs violet) so the two artifacts feel related.
const COLORS = {
  ink: "#1a1a1f",
  muted: "#6b6b76",
  faint: "#9a9aa4",
  hair: "#e3e3e8",
  studentAccent: "#0e7490",
  studentBg: "#ecfeff",
  studentBar: "#22d3ee",
  agentAccent: "#6d28d9",
  agentBg: "#f5f3ff",
  agentBar: "#a78bfa",
  danger: "#b91c1c",
  dangerBg: "#fef2f2",
};

const styles = StyleSheet.create({
  page: {
    paddingTop: 48,
    paddingBottom: 56,
    paddingHorizontal: 48,
    fontFamily: "Helvetica",
    fontSize: 10,
    color: COLORS.ink,
    lineHeight: 1.5,
  },
  header: {
    marginBottom: 20,
    paddingBottom: 14,
    borderBottomWidth: 1,
    borderBottomColor: COLORS.hair,
  },
  title: {
    fontFamily: "Helvetica-Bold",
    fontSize: 18,
    letterSpacing: -0.3,
  },
  subtitle: { marginTop: 4, fontSize: 10, color: COLORS.muted },
  metaRow: { marginTop: 10, flexDirection: "row", flexWrap: "wrap", gap: 16 },
  metaItem: { flexDirection: "column" },
  metaLabel: {
    fontSize: 7,
    letterSpacing: 1,
    textTransform: "uppercase",
    color: COLORS.faint,
  },
  metaValue: { fontSize: 9, color: COLORS.ink, fontFamily: "Helvetica-Bold" },
  turn: { marginBottom: 16 },
  turnHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    marginBottom: 6,
  },
  turnLabel: {
    fontSize: 8,
    letterSpacing: 1.2,
    textTransform: "uppercase",
    color: COLORS.faint,
    fontFamily: "Helvetica-Bold",
  },
  pill: {
    fontSize: 7,
    paddingHorizontal: 5,
    paddingVertical: 1.5,
    borderRadius: 3,
    color: COLORS.muted,
    backgroundColor: "#f1f1f4",
  },
  pillDanger: { color: COLORS.danger, backgroundColor: COLORS.dangerBg },
  bubble: {
    marginBottom: 6,
    paddingVertical: 7,
    paddingHorizontal: 10,
    borderLeftWidth: 3,
    borderRadius: 3,
  },
  studentBubble: {
    backgroundColor: COLORS.studentBg,
    borderLeftColor: COLORS.studentBar,
  },
  agentBubble: {
    backgroundColor: COLORS.agentBg,
    borderLeftColor: COLORS.agentBar,
  },
  speaker: {
    fontSize: 7.5,
    letterSpacing: 1,
    textTransform: "uppercase",
    fontFamily: "Helvetica-Bold",
    marginBottom: 3,
  },
  studentSpeaker: { color: COLORS.studentAccent },
  agentSpeaker: { color: COLORS.agentAccent },
  message: { fontSize: 10, color: COLORS.ink },
  footer: {
    position: "absolute",
    bottom: 24,
    left: 48,
    right: 48,
    flexDirection: "row",
    justifyContent: "space-between",
    fontSize: 7.5,
    color: COLORS.faint,
    borderTopWidth: 1,
    borderTopColor: COLORS.hair,
    paddingTop: 6,
  },
});

function studentLabel(actor?: string): string {
  switch (actor) {
    case "llm":
      return "Student · LLM";
    case "human":
      return "Student · Human";
    case "scripted":
      return "Student · Scripted";
    default:
      return "Student";
  }
}

export function TranscriptPdfDocument({
  turns,
  meta,
}: {
  turns: TranscriptTurn[];
  meta: TranscriptPdfMeta;
}) {
  return (
    <Document
      title={`Conversation — ${meta.agentLabel}`}
      author="AgentClash"
      subject={`Multi-turn transcript for ${meta.runName}`}
    >
      <Page size="A4" style={styles.page}>
        <View style={styles.header} fixed>
          <Text style={styles.title}>Conversation Transcript</Text>
          <Text style={styles.subtitle}>
            {meta.agentLabel} — {meta.runName}
          </Text>
          <View style={styles.metaRow}>
            <View style={styles.metaItem}>
              <Text style={styles.metaLabel}>Turns</Text>
              <Text style={styles.metaValue}>{turns.length}</Text>
            </View>
            <View style={styles.metaItem}>
              <Text style={styles.metaLabel}>Run</Text>
              <Text style={styles.metaValue}>{meta.runId.slice(0, 8)}</Text>
            </View>
            <View style={styles.metaItem}>
              <Text style={styles.metaLabel}>Agent</Text>
              <Text style={styles.metaValue}>{meta.runAgentId.slice(0, 8)}</Text>
            </View>
            <View style={styles.metaItem}>
              <Text style={styles.metaLabel}>Generated</Text>
              <Text style={styles.metaValue}>{meta.generatedAt}</Text>
            </View>
          </View>
        </View>

        {turns.map((turn) => (
          <View key={turn.turn_index} style={styles.turn} wrap={false}>
            <View style={styles.turnHeader}>
              <Text style={styles.turnLabel}>Turn {turn.turn_index}</Text>
              {turn.phase_id ? (
                <Text style={styles.pill}>{turn.phase_id}</Text>
              ) : null}
              {turn.mismatch ? (
                <Text style={[styles.pill, styles.pillDanger]}>mismatch</Text>
              ) : null}
              {turn.awaiting_human ? (
                <Text style={styles.pill}>awaiting human</Text>
              ) : null}
            </View>

            {turn.user_message ? (
              <View style={[styles.bubble, styles.studentBubble]}>
                <Text style={[styles.speaker, styles.studentSpeaker]}>
                  {studentLabel(turn.actor)}
                </Text>
                <Text style={styles.message}>{turn.user_message}</Text>
              </View>
            ) : null}

            {turn.assistant_message ? (
              <View style={[styles.bubble, styles.agentBubble]}>
                <Text style={[styles.speaker, styles.agentSpeaker]}>Agent</Text>
                <Text style={styles.message}>{turn.assistant_message}</Text>
              </View>
            ) : null}
          </View>
        ))}

        <View style={styles.footer} fixed>
          <Text>AgentClash · multi-turn conversation transcript</Text>
          <Text
            render={({ pageNumber, totalPages }) =>
              `${pageNumber} / ${totalPages}`
            }
          />
        </View>
      </Page>
    </Document>
  );
}
