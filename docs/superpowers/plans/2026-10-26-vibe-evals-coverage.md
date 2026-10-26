# Vibe Evals audit coverage

Primary ownership for all 76 raw findings. Overlapping findings share a repair; completion requires implementation and verification, not merely assignment. Product proposals D01–D07 are covered by Tasks 3, 2, 6–7, 8, 8, 9, 10 respectively.


Tasks 1–2 are implemented and independently reviewed through `21b0f913` and `8d92f2eb`. Outcomes below mean verified local code/fixtures, not production restoration. Task 2 evidence includes the full 23-file/253-test frontend gate before its scoped recovery fixes, final 111-test affected-owner gate, final typecheck, actual child-database workspace query tests and split browser runs. The mandatory 139-test backend floor passed on a fresh fixture; Task 10 still owns final integrated repetition and fixture-isolation repair. Real Mac/physical-device and production authentication/provider/funding checks remain external.

| Audit ID | Finding | Tasks | Verified outcome |
|---|---|---|---|
| BUILD-01 | Active Build entry is shown while hosted execution is disabled | 1, 2 | Availability gates both doors; deterministic worker journey passes. Production setup remains operator dependent. |
| BUILD-02 | Setup error gives the visitor no recovery action | 1, 2 | Setup retry, read-only example, history and Back remain usable. |
| BUILD-03 | Workspace provides no discoverable sign-in or account identity route | 1, 2 | Account identity and sign-in route are exposed. |
| BUILD-04 | Unavailable capabilities are described as available | 1, 2 | Capability labels and actions respect actual availability. |
| BUILD-05 | Empty model inventory is presented as a selectable default model | 1, 2 | An empty catalog offers no invented model. |
| BUILD-06 | Saved-list request failures silently become empty states | 1, 2 | Real workspace filtering, distinct list states and retained rows on transient failure. |
| BUILD-07 | Saved-work empty instructions reference unavailable or inconsistent actions | 1, 2 | Saved-list guidance matches available actions. |
| BUILD-08 | Initial Build quote cost is fetched but not shown | 2 | Build estimate shows maximum price, funding and scope. |
| BUILD-09 | Quote lifecycle can strand the first-build send action | 2 | Server-clock quote states and nonexecuting refresh preserve the next action. |
| BUILD-10 | Input limits use different units in UI and API | 2 | Client uses advertised UTF-8 byte limits, including exact multibyte boundaries. |
| BUILD-11 | Upload dialog explains task use but not transfer/retention before submission | 3 | Pending |
| BUILD-12 | Attachment blockers are hidden behind Manage attachment | 3 | Pending |
| BUILD-13 | Material reading has no distinct stalled or delayed state | 3 | Pending |
| BUILD-14 | Material dialog initially selects a known unavailable method | 3 | Pending |
| BUILD-15 | Closed attachment chips omit how each source will be used | 3 | Pending |
| BUILD-16 | Policy adoption requires manual exact copying from a separate text block | 3 | Pending |
| BUILD-17 | Successful paste attachment leaves an active repeat-submit draft | 2, 3 | Pending |
| BUILD-18 | Material paste draft is excluded from the existing composer restoration | 2, 3 | Pending |
| BUILD-19 | Build trial hides reset and trial history controls | 2 | Trial dock exposes New and history, including during inference outages. |
| BUILD-20 | Different trial threads blend into one unlabeled timeline | 2 | Interleaved trial threads have explicit boundaries. |
| BUILD-21 | Prominent contact action leads to a copy-only dead end when contact is disabled | 1, 2 | Contact action follows configured form or mail capability. |
| BUILD-22 | Cancellation removes all per-version next actions | 4 | Pending |
| BUILD-23 | One missing source hides all otherwise readable material | 3 | Pending |
| BUILD-24 | A single historic evidence failure blocks the entire session export | 5 | Pending |
| E01 | Anonymous evaluation cannot start | 1, 2 | Guest availability is explicit; deterministic guest execution passes. Live funding/config is not asserted. |
| E02 | Test edits disappear on a Results round trip | 2 | Drafts survive remount/reload; same-version history and async version selection are verified. |
| E03 | Choose observed replies versus a text surrogate explicitly | 3 | Pending |
| E04 | Keep the expectation visible before running a generated test | 3, 4 | Pending |
| E05 | Blocked test validation needs a reason and next action | 3, 4 | Pending |
| E06 | Disclose target-model changes in evaluation comparisons | 4 | Pending |
| E07 | Explain remote processing before pasting agent instructions | 3 | Pending |
| E08 | Progress should name the text prototype rather than the live agent | 4 | Pending |
| E09 | Compare previous and revised evidence for each changed case | 4 | Pending |
| E10 | Make unresolved work actionable without repeating every successful case | 4 | Pending |
| E11 | Count failing examples rather than inferred fixes | 4 | Pending |
| E12 | Expose failure and unresolved filters above collapsed results | 4 | Pending |
| E13 | Keep a stable result heading when evidence arrives | 4 | Pending |
| E14 | Separate a disputed grade from a changed business rule | 4 | Pending |
| E15 | Provide a direct path from excerpt to full explanation | 4 | Pending |
| E16 | Refresh expired run quotes before confirming | 2 | Run estimates expire on the server clock and refresh without execution. |
| E17 | Make Stop consequences predictable | 4 | Pending |
| E18 | Differentiate a provider wait from stale progress | 4 | Pending |
| E19 | Name export contents and scope before downloading | 5 | Pending |
| E20 | Connect preserved complex packs to an actionable next step | 5 | Pending |
| A01 | Return submits instead of allowing touch users to compose multiline instructions | 2 | Touch/hybrid Return inserts a line; desktop, modifiers and composition have transport regressions. |
| A02 | Portaled dialog animation ignores Reduce Motion | 3 | Pending |
| A03 | Dialog close targets are smaller than other touch controls | 3 | Pending |
| A04 | Long Settings puts the only visible close control inside the scrolling content | 3 | Pending |
| A05 | Settings uses unusually small essential explanatory text on phones | 3 | Pending |
| A06 | The workspace ignores the system light appearance preference | 3 | Pending |
| A07 | Active context uses an aria-label on an element without naming semantics | 3 | Pending |
| A08 | Capabilities repeat the generic link name Documentation | 3 | Pending |
| A09 | Saving or cancelling a test edit removes the focused control without restoring task focus | 2 | Save/Discard restore task focus; asynchronous source acknowledgement preserves newer drafts. |
| A10 | Jump to New response moves keyboard focus back to the composer | 2, 3 | Pending |
| A11 | Long case replies create a second vertical scrolling region | 3 | Pending |
| A12 | Truncated agent titles rely on hover-only title attributes for their full name | 5 | Pending |
| A13 | Attachment completion states have no explicit live announcement | 3 | Pending |
| A14 | Virtual keyboard visibility has not been validated for the fixed-height workspace | 3 | Pending |
| A15 | Text growth can exceed generic dialog and nowrap action constraints | 3 | Pending |
| A16 | Resting editable/control frames have little visual separation from their backgrounds | 3 | Pending |
| R01 | Fresh visitors cannot start either journey | 1, 2 | Truthful entry availability and deterministic execution verified; live restoration needs operator values. |
| R02 | Disabled execution still presents active entry actions | 1, 2 | Unavailable inference blocks new execution while historical actions remain usable. |
| R03 | History failures and authentication states look like empty work | 1, 2 | Signed-out, forbidden, empty and failed states differ; rows are actor/workspace scoped. |
| R04 | Model selectors imply choices despite an empty model catalog | 1, 2 | Real inventory is required for selectable models. |
| R05 | Capability documentation obscures availability | 1, 2 | Capability availability and uniquely named documentation are visible. |
| R06 | Object names vary across navigation, saving and deletion | 3, 5 | Pending |
| R07 | Meaningful view changes replace browser history | 2 | Meaningful navigation pushes; hydration replaces; case/thread Back/Forward and request recovery verified. |
| R08 | Entry shell lacks a visible help/account/home route | 1, 2 | Account, Home, Help and saved-project navigation are exposed. |
| R09 | Collections do not scale beyond a few projects | 5 | Pending |
| R10 | Sidebar collapse preference resets on reload | 5 | Pending |
| R11 | Journey analytics omit entry failure and the evaluation path | 1, 2 | Both doors track attempts and sanitized preadmission failures, including create/cookie/estimate failures. |
| R12 | Sensitive-input processing facts are absent at the intake boundary | 3 | Pending |
| R13 | Save handoff needs an explicit object and destination summary | 5 | Pending |
| R14 | Destructive dialog does not name the project | 5 | Pending |
| R15 | Contextual docs use controls absent from current interface | 5 | Pending |
| R16 | Measure route cost before adding heavier UI | 10 | Pending |
