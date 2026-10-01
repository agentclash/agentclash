These fixtures retain the diagnosed 15 September 2026 failures without account,
session, credential, or billing records. `returns-original-request.txt` preserves
the original bytes, including blank lines and its final newline.

`captured-failures.json` contains the original task text and recorded provider
outputs for the multiline quote/empty reply failure, invented refund assurance,
five-requested/three-generated count mismatch, and 30-day-input/14-day-expectation
contradiction. The last fixture also retains the preceding and resulting pack
blueprints. Provider outputs are evidence to replay; they are not trusted commands.

Deterministic tests can check parser, patch, persistence, and gate behavior using
these records. A scripted critic verdict does not measure model semantic accuracy.
