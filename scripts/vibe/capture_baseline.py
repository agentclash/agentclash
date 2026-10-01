#!/usr/bin/env python3
"""Export allowlisted terminal-turn events from local Vibe; never read chat text.

No writes, inference, credentials in output, or claims of semantic understanding.
Elapsed time is admission-to-completion including queue time, not first-token or
browser rendering latency. Cost null remains unknown. Raw operation IDs stay in
the private event artifact, never aggregate metric labels.
"""
import argparse
import datetime
import json
import math
import os
from pathlib import Path
import statistics
import subprocess
from urllib.parse import urlparse
from db_env import connection_env

KINDS={"message","build","check","retest","playground"}
STATES={"COMPLETED","FAILED","PARTIAL","CANCELLED","EXPIRED"}
FIELDS={"operation_id","authoring_version","kind","state","turn_ms","cost_nano_usd","model_calls"}

def summarize(events):
    seen=set(); groups={}
    for e in events:
        if set(e)!=FIELDS or e["operation_id"] in seen or e["kind"] not in KINDS or e["state"] not in STATES:
            raise ValueError("invalid/duplicate/private event fields")
        seen.add(e["operation_id"])
        if type(e["authoring_version"]) is not int or not 0<=e["authoring_version"]<=10000 or type(e["model_calls"]) is not int or e["model_calls"]<0:
            raise ValueError("invalid version/call count")
        for k in ("turn_ms","cost_nano_usd"):
            if e[k] is not None and (type(e[k]) not in {int,float} or not math.isfinite(e[k]) or e[k]<0):
                raise ValueError("invalid measurement")
        groups.setdefault(str(e["authoring_version"])+"/"+e["kind"],[]).append(e)
    result={}
    for key,rows in groups.items():
        ms=sorted(e["turn_ms"] for e in rows if e["turn_ms"] is not None)
        result[key]={"turns":len(rows),"states":{s:sum(e["state"]==s for e in rows) for s in sorted(STATES)},
                     "p50_turn_ms":statistics.median(ms) if ms else None,"p95_turn_ms":ms[max(0,math.ceil(.95*len(ms))-1)] if ms else None,
                     "unknown_latency_turns":len(rows)-len(ms),"known_cost_nano_usd":sum(e["cost_nano_usd"] for e in rows if e["cost_nano_usd"] is not None),
                     "unknown_cost_turns":sum(e["cost_nano_usd"] is None for e in rows),"model_calls":sum(e["model_calls"] for e in rows),
                     "unnecessary_questions":None,"useful_suite_completion":None}
    return result

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument("--output",type=Path,required=True);p.add_argument("--limit",type=int,default=200)
    args=p.parse_args()
    if not 1<=args.limit<=1000:p.error("limit must be 1–1000")
    dsn=os.environ.get("VIBE_BASELINE_DATABASE_URL","");url=urlparse(dsn)
    if url.scheme not in {"postgres","postgresql"} or url.hostname not in {"localhost","127.0.0.1","::1"}:p.error("VIBE_BASELINE_DATABASE_URL must be an explicit local connection")
    query="""BEGIN READ ONLY;
SET LOCAL statement_timeout='10s';
SELECT COALESCE(json_agg(row_to_json(e)),'[]'::json) FROM (
 SELECT id::text AS operation_id, COALESCE((input->>'authoring_version')::integer,0) AS authoring_version,
 kind,state,EXTRACT(epoch FROM (completed_at-created_at))*1000 AS turn_ms,
 actual_cost AS cost_nano_usd,model_calls FROM vibe_operations
 WHERE completed_at IS NOT NULL AND state IN ('COMPLETED','FAILED','PARTIAL','CANCELLED','EXPIRED')
 ORDER BY created_at DESC LIMIT :limit
) e;
COMMIT;
"""
    env=connection_env(dsn)
    r=subprocess.run(["psql","-X","-qAt","-v","ON_ERROR_STOP=1","-v","limit="+str(args.limit)],input=query,capture_output=True,text=True,env=env)
    if r.returncode:raise SystemExit("Read-only baseline query failed; connection details withheld")
    events=json.loads(r.stdout);summary=summarize(events)
    report={"version":1,"captured_at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"source":"local saved operations; observational, not a controlled model comparison", "sample_limit":args.limit,
            "annotation_status":"unnecessary questions and useful suite quality require independent human annotation; null is not zero", "events":events,"summary":summary}
    fd=os.open(args.output,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,"w") as f:json.dump(report,f,indent=2);f.write("\n")
    print(json.dumps({"turns":len(events),"summary":summary},indent=2))

if __name__=="__main__":main()
