import { expect, it } from "vitest";
import { VibeError } from "./vibe";
import { recoverRequest } from "./vibe-request-recovery";
it("unlocks only a first proven rejection",()=>{
 const request={body:'{"client_id":"original"}',uncertain:false};
 expect(recoverRequest(request,new VibeError("input_unavailable","Replace it",409,"rejected"))).toBe("rejected");
 expect(request.uncertain).toBe(false);
});
it("uncertainty remains sticky after later rejection and restoration",()=>{
 const request={body:'{"client_id":"original"}',uncertain:false};
 expect(recoverRequest(request,new Error("Connection lost"))).toBe("uncertain");
 expect(recoverRequest(request,new VibeError("forbidden","Auth",403,"rejected"))).toBe("uncertain");
 expect(recoverRequest(request,new VibeError("invalid_input","Validation",400,"rejected"))).toBe("uncertain");
 expect(request.body).toBe('{"client_id":"original"}');
 expect(recoverRequest({body:request.body,uncertain:true},new VibeError("not_found","Missing",404,"rejected"))).toBe("uncertain");
});
it("missing metadata and identity conflicts cannot authorize a replacement ID",()=>{
 expect(recoverRequest({body:"original",uncertain:false},new VibeError("invalid_input","Old server",400))).toBe("uncertain");
 expect(recoverRequest({body:"original",uncertain:false},new VibeError("idempotency_conflict","Existing ID",409,"rejected"))).toBe("uncertain");
});
