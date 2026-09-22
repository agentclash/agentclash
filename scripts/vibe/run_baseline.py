#!/usr/bin/env python3
"""Run the committed Vibe regression floor; missing/skipped DB tests are failures."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
from urllib.parse import urlparse

ROOT=Path(__file__).resolve().parents[2]
REQUIRED={
 "github.com/agentclash/agentclash/backend/internal/vibe": [
  "TestRequirementProvenanceAndReplacement", "TestVibePendingRequirementRevisionAndManualOverride",
  "TestVibeContextDiagnostics", "TestVibeExistingAgentAndSupportBoundaries",
  "TestVibeAuthoringRejectsFalseProvenanceAndInternalConfiguration",
  "TestIntegrationDuplicateMessageAndRevision", "TestIntegrationAtomicWalletReservations",
  "TestIntegrationCrashKeepsJournaledEvidenceWithoutReexecution",
  "TestIntegrationStopMidProviderAndNoRetry", "TestIntegrationAuthoringIncludesActiveAcceptedArtifact",
  "TestVibeIntegrationReviewedHandoffAndAtomicCompletion"],
 "github.com/agentclash/agentclash/backend/internal/api": ["TestVibeIntegrationDescriptionToHonestScorecardAndSave"],
 "github.com/agentclash/agentclash/backend/internal/vibe/interaction": [
  "TestContractsAcceptSupportedWireData", "TestContractsRejectAmbiguousOrAuthorityBearingData",
  "TestContractsRejectMalformedAndFutureVersions"],
}

def check_events(events, returncode):
    outcomes={(e.get("Package"),e.get("Test")):e.get("Action") for e in events if e.get("Action") in {"pass","fail","skip"}}
    problems=[]
    if returncode: problems.append("go test failed")
    if any(e.get("Action")=="fail" for e in events): problems.append("failed test/package event")
    if any(e.get("Action")=="skip" for e in events): problems.append("a required test/subtest was skipped")
    for package,names in REQUIRED.items():
        for name in names:
            outcome=outcomes.get((package,name),"missing")
            if outcome!="pass": problems.append(name+": "+outcome)
    return problems

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument("--report",type=Path,required=True);args=p.parse_args()
    dsn=os.environ.get("VIBE_TEST_DATABASE_URL","")
    parsed=urlparse(dsn)
    if not dsn or parsed.scheme not in {"postgres","postgresql"} or parsed.hostname not in {"127.0.0.1","localhost","::1"} or not parsed.path.lstrip("/").startswith("vibe_test"):
        p.error("VIBE_TEST_DATABASE_URL must name an isolated, migrated local vibe_test database")
    # Do not silently replace a previous result or accidentally run paid tests.
    with args.report.open("x") as output:
        os.chmod(args.report,0o600)
        regex="^("+"|".join(name for names in REQUIRED.values() for name in names)+")$"
        env=dict(os.environ)
        for k in list(env):
            if k.startswith("VIBE_LIVE_"): env.pop(k)
        process=subprocess.run(["go","test","-p","1","-race","-json","-count=1","-timeout=8m","./internal/vibe","./internal/api","./internal/vibe/interaction","-run",regex],cwd=ROOT/"backend",env=env,capture_output=True,text=True)
        events=[]
        for line in process.stdout.splitlines():
            try: events.append(json.loads(line))
            except ValueError: pass
        problems=check_events(events,process.returncode)
        report={"version":1,"mode":"deterministic fake inference; no model accuracy claim","required_tests":sum(map(len,REQUIRED.values())),"passed":not problems,"problems":problems,
                "events":[{k:e[k] for k in ("Action","Package","Test","Elapsed") if k in e} for e in events if e.get("Action") in {"pass","fail","skip"}]}
        json.dump(report,output,indent=2);output.write("\n")
    print(json.dumps({k:report[k] for k in ("required_tests","passed","problems")},indent=2))
    if problems:
        # Toolchain failures need a diagnostic; do not echo DB passwords/DSNs.
        diagnostic=process.stderr.replace(dsn,"[database]")
        if parsed.password: diagnostic=diagnostic.replace(parsed.password,"[redacted]")
        print(diagnostic[-4000:],file=sys.stderr)
        raise SystemExit(1)

if __name__=="__main__":main()
