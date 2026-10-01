import type { Operation } from "@/lib/vibe";

export function recoveryMessage(operation: Operation) {
  switch (operation.error?.code) {
    case "provider_timeout":
    case "usage_unknown":
    case "worker_interrupted":
      if (operation.billing === "RECONCILING")
        return "The response was interrupted. Your request is saved. We’re checking whether the provider finished before another attempt is allowed.";
      break;
    case "provider_auth":
    case "hosted_disabled":
    case "pricing_unavailable":
    case "accounting_unavailable":
      return "The AI connection needs attention from the person running Vibe Evals. Your request is saved; sending it again won’t fix the connection.";
  }
  if (operation.error?.code === "invalid_response" && operation.error.message.includes("tests are saved"))
    return "I couldn’t complete that response. Your request is still here.";
  return operation.error?.message;
}
