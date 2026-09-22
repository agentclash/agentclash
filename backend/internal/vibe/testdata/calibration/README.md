`suite-validity.json` contains 72 proposed reference labels across returns, sales,
scheduling, extraction, routing, and formatting. Twelve cases are for calibration;
60 are held out, with 20 supported, 20 contradicted, and 20 unclear examples.

These labels were machine-authored during implementation. They require independent human
review before being described as human-calibrated evidence. A passing fixture
manifest test establishes complete data and distinct splits, not model accuracy.
Eight arithmetic or enum relations also have explicit reference operands checked
independently in Go. Their source transcription still needs human review, and those
checks do not certify the remaining meaning of the natural-language cases.

Each case has its original policy source, current request, input, expected behavior,
shared criteria, reference status and reason. Review the source interpretation as
well as the expectation. Do not show reference labels/reasons or the split to the
model. Do not tune prompts using held-out results and keep calling them held out.

The full dataset costs at most 72 single-candidate review calls with no automatic
repair. Under an 80-call overall budget, reserve the remaining calls explicitly for
other tests, or choose and record a smaller fixed review subset before starting.
Count failures and abstentions, including malformed/provider-failed reviews; never
retry until green. Report sample sizes, false acceptance/rejection, abstention,
coverage, and per-domain outcomes separately from deterministic test results.
