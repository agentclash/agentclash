import { vibeFetch } from "./vibe";

export type InputBinding = { input_id: string; content_hash: string; usage: "task_input" | "reference"; accept_partial?: boolean };
export type DocumentSource = { input_id: string; hash: string; page: number; exact_quote: string };
export type TaskMaterial = {
  id: string; session_id: string; client_id: string; kind: "text" | "pdf"; name: string;
  status: "uploaded" | "extracting" | "ready" | "unreadable" | "failed" | "deleted" | "expired";
  content_hash: string; warnings: string[]; error?: string; expires_at?: string;
  page_count?: number; pages?: { number: number; text: string }[];
};
export const inputPath = (session: string, id?: string) => `/sessions/${session}/inputs${id ? `/${id}` : ""}`;
export function uploadMaterial(session: string, client: string, value: File | string, token?: string | null) {
  let body: BodyInit;
  if (typeof value === "string") body = JSON.stringify({ client_id: client, kind: "text", text: value });
  else { const form = new FormData(); form.set("client_id", client); form.set("file", value); body = form; }
  return vibeFetch<TaskMaterial>(inputPath(session), token, { method: "POST", body });
}
