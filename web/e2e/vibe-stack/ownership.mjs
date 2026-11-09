import { open } from "node:fs/promises";
export async function recordOwned(kind, name, details = {}) {
  if (!process.env.VIBE_BROWSER_OWNERSHIP_LOG) return;
  const f=await open(process.env.VIBE_BROWSER_OWNERSHIP_LOG,"a",0o600);
  try {await f.writeFile(JSON.stringify({kind,name,pid:process.pid,time:new Date().toISOString(),phase:"intent",...details})+"\n");await f.sync();} finally {await f.close();}
}
