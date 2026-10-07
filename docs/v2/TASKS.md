# v2 task cards

> **September 18 owner update:** G1 is conditionally accepted for local implementation through the full release candidate. Historical benchmark failures stay recorded; separate 2 GiB/one shared CPU Forge is the planning target. The owner operates all hosting, with no agent account/remote access. Local work continues through G2/M3 preparation; owner playthrough, hosted acceptance and enablement remain separate pending checklists. No merge/push/deploy/purchase is authorized. See STATUS.md.

Task order, dependencies, ownership, and state live in [STATUS.md](STATUS.md). Acceptance below is cumulative with [VALIDATION.md](VALIDATION.md); a card cannot waive a release gate. Each card should normally result in a small reviewable change or evidence report; compatible changes can share a milestone PR. Split a larger card into named subtasks when needed, without quietly widening it. IDs are stable, so follow ledger dependencies rather than numeric order.

## M0 — Establish baseline

### V2-001 — Baseline and current product flows

- **Deliver:** a concise current-flow document linked to [issue #49](https://github.com/quehorrifico/manatomb-v3/issues/49), current commit/production-version comparison, and baseline verification results.
- **Inspect:** `internal/web/ROUTES.md`, deck handlers, `internal/web/assets/playtest.js`, relevant templates, `docs/releases.md`, CI, deployment configuration available without secrets.
- **Accept:** document guest builder, Commander creation, sandbox, import, account/save/publish, public deck, analytics, and goldfish entry/return. Distinguish saved account data, local draft data, and ephemeral game state. Record exactly where CPU launch can reuse data and where separate session behavior is necessary.
- **Check:** run existing relevant deck/playtest tests and record full baseline CI availability; reproduce the important flows in a local browser where setup permits. Mark environmental blockers explicitly. No UI redesign or new game code.

### V2-002 — Reproduce and classify open issues

- **Deliver:** refresh `BUGS.md` from all open repository issues, preserving linked issue titles and splitting bundled reports into local task IDs.
- **Accept:** each candidate has reproduction steps, expected/actual behavior, affected data/browser, severity, v2 disposition, and evidence or an explicit “not reproduced.” Check if current code already fixes any report. The snapshot is not proof of a bug.
- **Check:** establish focused regressions for required bugs when fixing them, not speculative tests for every report. Do not treat enhancement requests as mandatory blockers.

## M1 — Prove Forge before product UI

### V2-010 — Pin/build Forge and catalog adapter seams

- **Deliver:** a pinned upstream revision, reproducible headless build/run instructions, dependency/resource inventory, and a human-input interface map in a reusable evidence report.
- **Accept:** build from the pin and initialize card/edition resources without a graphical desktop; execute a Commander AI simulation as a baseline. Identify the real human-controller, view, and prompt interfaces; inventory prompt families and their threading/blocking behavior. Record license/notices for the code/resources actually used and the source-publication/attribution steps to implement. Do not infer obligations are avoided by a service boundary.
- **Check:** record Java/Maven versions, engine/config/resource hashes, exact command, build result, image/runtime size. If initialization still needs GUI classes, document why and the minimal adapter path. A successful desktop launch is insufficient.
- **Stop:** after a bounded build attempt and one targeted correction, report unresolved dependencies with evidence; do not repeatedly build unrelated Forge modules.

### V2-011 — Human/CPU browser feasibility proof

- **Deliver:** a disposable browser harness and thin bridge, using actual Forge human prompts and CPU decisions.
- **Accept:** a human can keep/mulligan, take legal actions, respond to a target/payment/choice prompt, pass priority, declare attacks/blocks, and observe CPU actions and engine-generated state. Include a stack response and commander movement/recast in the proof corpus. Return errors for invalid/stale input; serialize each game's mutations. Record the exact interfaces used and gaps discovered.
- **Check:** capture a sanitized transcript/video and a repeatable short scenario, plus a game reaching Forge's natural result. A small fixture may accelerate the game, but include a realistic Commander deck scenario for resource testing. No hard-coded state transitions, mocked CPU, or simulated UI answers in the human proof.
- **Stop:** no full ManaTomb board styling. If the only path requires rebuilding card logic or broad upstream surgery, write the blocker and narrow alternatives for G1.

### V2-012 — Resource envelope and lifecycle

- **Deliver:** the completed benchmark table and failure observations described in `TECHNICAL_PLAN.md`.
- **Accept:** Linux AMD64/container-capped measurements for startup, one active game, idle human prompt, demanding CPU turns, repeated games, cancel, reconnect, worker loss, and reclamation. Record whole-process/container memory, not just Java heap. Compare a 512 MiB cap and then 1/2 GiB only as necessary, locally. No paid instance is required for this task.
- **Check:** a fixed representative corpus with repeat runs; same shared-CPU-equivalent limit; identify warm/cold start time, decision latency, live-game memory, lingering threads and eventual memory plateau. Demonstrate terminating a stuck worker releases capacity. Document how assumptions must be confirmed on actual hosting.
- **Stop:** do not turn this into concurrency tuning. If one safe game cannot fit at the highest local candidate, present the measured failure and cost implications; do not silently select a larger paid plan.

### V2-013 — Feasibility decision and first contract

- **Deliver:** G1 report with pass/conditional-pass/fail, smallest viable topology, cost candidates, known input/view gaps, and a versioned first session contract.
- **Accept:** V2-010–012 evidence supports a human-controlled browser match; no required custom card logic; one-session isolation and resource reclamation are demonstrated; remaining work is identifiable adapter/UI work. Specify transport, ownership, hidden-information filtering, command IDs/revisions, cancellation, and engine-version identity.
- **Owner gate:** present actual evidence and exact proposed additional monthly cost/concurrency. Owner accepts or revises the implementation/hosting direction. A local feasible build can pass the technical part while paid deployment remains pending. Do not start M2 while the overall G1 decision is unresolved.

## M2 — Internal MVP

### V2-020 — Session and deck input contracts

- **Deliver:** compact request/event/response schemas and fixtures grounded in M1, including setup, prompts, acknowledgments, views, errors, and terminal results.
- **Accept:** identify the human seat, actor/owner, session incarnation, command id, revision, and prompt id. Forge supplies the actual choices and legality. Define deck canonicalization, commander section(s), print metadata, and errors without duplicating engine rules.
- **Check:** contract examples for valid action, stale prompt, repeated command, uncertain acknowledgment, unauthorized session, unavailable card, and engine failure. Audit ManaTomb's single persisted commander field versus Forge's possible multi-commander decks; choose a setup-level representation without requiring an unrelated editor rewrite. Carry through supported partner/background arrangements or record an explicit owner-approved limitation before final v2.

### V2-021 — Forge service and human prompt bridge

- **Deliver:** a separately runnable Java adapter using the pinned Forge build with a small reproducible test harness.
- **Accept:** create a Commander game from deck data, execute CPU turns, expose only the human-allowed view, route every inventoried MVP prompt, deliver acknowledgments and stable prompt/revision identities, and report natural result/concede/error. Inventory unimplemented prompt families explicitly.
- **Check:** integration scenarios for concurrent/duplicate input, hidden zones including face-down cards, engine exceptions, prompt cancellation, and termination during CPU work. Use the isolation model proven in M1. Never depend on browser-side filtering or invent a legal-action menu by parsing card text.

### V2-022 — Go session ownership and admission

- **Deliver:** CPU routes/service module integrated with existing authentication and a disabled-by-default feature flag.
- **Accept:** signed-in ownership checked on create/read/action/reconnect/end; server-side limits on deck payloads and request rates; one active match per account and initial global cap one; duplicate creates cannot reserve two slots. Full capacity returns an honest busy response without an unbounded queue. Browser actions use same-origin/CSRF protections; engine access is private and authenticated.
- **Check:** requests for another account's session fail, untrusted clients cannot call the engine directly, state-changing GETs are rejected, stale workers cannot revive expired sessions, and admission is released after every terminal path. Forge failure must not make ordinary site health/deck browsing fail.

### V2-023 — Custom decks and provisional opponent

- **Deliver:** saved/pasted player deck setup, pasted CPU deck setup, and one provisional default opponent.
- **Accept:** preserve an immutable match-start deck snapshot; changes to a saved deck midgame do not mutate the game. Display import/mapping/engine-validation errors before reserving long-lived capacity. Pass both decks and commander sections to Forge. Exclude maybeboard/sideboard content unless the selected Forge configuration explicitly uses it.
- **Check:** known valid deck, unknown/ambiguous name, different printing, double-faced/split name, invalid quantity/size, commander duplicated in the main section, unsupported card, and a Forge-supported multi-commander fixture. Do not fetch arbitrary URLs or allow custom executable card files. Deck data is not published by starting a CPU match.

### V2-024 — Functional browser match UI

- **Deliver:** a simple complete desktop play loop through existing ManaTomb page patterns.
- **Accept:** setup → start → mulligan → turns/priority → mana/payment/target/choices → combat → engine result or concede → return. Show phase, priority owner, stack, visible zones, life/commander status and available choices. A submitted action stays pending until acknowledged; reconnect/resync resolves uncertainty.
- **Check:** complete a natural game using a custom player deck and a custom CPU deck, and another against the default. Execute all MVP corpus prompts with real input. A display-only board, click-through mock, or concede-only finish does not count.

### V2-030 — MVP reconnect, visibility, and cleanup

- **Deliver:** bounded reconnection, clear ended/expired/error states, and correct cleanup at all session exits.
- **Accept:** refresh and a short disconnection resume the same living engine session without replaying actions. Resolve a second tab with a clear controlling-tab policy. Grace, idle, and total-lifetime timers are server-owned and configurable; proposed starting values are in `TECHNICAL_PLAN.md`. Warn before timeouts. Never auto-answer human decisions on timeout.
- **Check:** dropped acknowledgment after an accepted action, stale browser revision, authorization expiry, tab closure, idle timeout, concede while CPU runs, and engine restart. Hidden-state and command-ownership checks pass. Engine restart ends the game honestly; durable resume is not promised.

### V2-031 — Internal MVP acceptance (G2)

- **Deliver:** filled MVP checklist with reproducible test decks, pinned versions, limitations, and a short owner playthrough.
- **Accept:** all M2 work is integrated with owner approval; full match, custom opponent, one default, reconnect and cleanup demonstrated; no game-rule substitutions; no required prompt/hidden-state blockers in the MVP corpus; measured cap enforced.
- **Owner gate:** owner records the MVP as accepted or names concrete fixes. Start M3 only after acceptance; do not label the internal MVP as the public v2 release.

## M3 — Complete the public feature

### V2-040 — Five default CPU decks

- **Deliver:** exactly five versioned, inspectable Commander lists with names, commanders, short play-style descriptions, provenance, deck hashes, and tested Forge pin.
- **Accept:** all pass Forge validation and CPU-led playtesting; cover distinct understandable styles; avoid defaults dominated by known AI failures. Keep the provisional deck only if it meets the same standard.
- **Check:** each list runs in both seats where supported; at least one browser human-vs-CPU match per CPU default reaches a natural result. Automated AI simulations may add breadth but cannot replace the human interaction check. Do not claim difficulty calibration or flawless strategy.

### V2-041 — Builder and saved-opponent integration

- **Deliver:** CPU entry alongside goldfishing for saved decks and signed-in unsaved drafts, owned saved CPU-deck picker, rematch, and return-to-editor flow.
- **Accept:** starting/rematching requires no save of a draft; setup consumes an immutable snapshot. Returning preserves the original editor draft and printing choices, and playing cannot overwrite saved deck contents. Communicate local-versus-account save state. Resolve the multi-commander representation from V2-020.
- **Check:** guest-to-login draft handoff if offered, unsaved edits, missing saved deck, owned/private deck authorization, custom CPU selection, rematch capacity release, and existing goldfish entry/return paths. Do not implement remote deck-site URL integrations or the entire #64 editor import enhancement as a side effect.

### V2-042 — Desktop/tablet and accessible interaction

- **Deliver:** finished board, setup, prompt components, card inspection, public log, status feedback, and responsive styling using existing theme primitives.
- **Accept:** supported viewports have readable hand/stack/zones and operable prompts; touch has an alternative to drag/hover; keyboard focus stays visible and dialogs are usable; slow CPU turns show progress without pretending to know a completion percentage. Any yield shortcut invokes verified Forge behavior and can be disengaged.
- **Check:** the browser/device matrix in VALIDATION.md, keyboard-only targeting/choice/combat, reduced motion, long names/text, many permanents/tokens, and all theme variants. Phone visitors get readable setup/help and a clear supported-device message.

### V2-043 — Broader prompt coverage and diagnostics

- **Sequence:** after the MVP and before V2-040 default-deck acceptance and V2-042 final UI polish. Extend functional prompt components here so deck verification cannot be blocked by work scheduled later.
- **Deliver:** final prompt-family coverage matrix, compatibility note, outcome summary, and sanitized downloadable report.
- **Accept:** cover the inventoried Forge human-input surface reachable in supported Commander play, including complex choices, ordering, X/mana, replacement effects and combat assignment; document actual exceptions. Broad custom decks are accepted subject to engine catalog/validation, not a curated player-deck allowlist. Missing adapter prompts are ManaTomb release defects; upstream card bugs stay distinct.
- **Check:** a broader fixture corpus beyond the five defaults, uncommon commander structures, tokens/copies/transform/hidden cards, and error/hang cases. A generic prompt UI is acceptable only if it faithfully captures the original options and constraints. Report includes versions, visible action context and opaque error ID; no hidden game state or private deck data by default.

### V2-044 — Hosting and operational controls

- **Deliver:** reproducible deployment configuration, health/readiness split, access flag/cap controls, session drain/termination behavior, retention limits, metrics, and rollback instructions.
- **Accept:** use the smallest measured accepted topology; private engine endpoint; resource/decision ceilings; no in-game rules fallback; bounded logs, threads, temporary files, and DB records. Build pinned resources into deployment artifacts; no dependence on persistent local filesystem or downloading card art to the engine.
- **Check:** startup/readiness, actual proxy reconnect behavior, graceful shutdown, cold restart, missing/unhealthy engine, admission contention and return of capacity. Prepare the exact cost and app configuration for approval before any paid provisioning. Reconfirm local measurements on the approved target before public access.

### V2-045 — Close v2 bug scope

- **Deliver:** final bug disposition and regression evidence for #64 items and new v2 reports.
- **Accept:** required reproducible Commander-selection and save-state issues fixed or shown already fixed with evidence; no outstanding unaccepted release blockers; optional/deferred improvements remain distinguishable. Refresh GitHub issue state before declaring completion.
- **Check:** BUG-064A/B acceptance below and any new issue's targeted reproduction. Do not close #64 solely because one item is resolved. Required bug fixes may ship earlier as v1 patches and be linked here.

## M4 — Validate and release

### V2-050 — Release candidate verification

- **Deliver:** completed final checklist, capacity/soak results, and regression evidence tied to the actual candidate commits, Forge pin and configuration.
- **Accept:** VALIDATION.md gates pass at the configured capacity on the approved target or faithful staging environment; no unexplained failures; any limitation is explicit. Broad MTG correctness is not claimed from a finite corpus.
- **Check:** current required CI, meaningful adapter tests, browser journeys, fault injection, boundary/capacity checks, and existing product regressions. Do not repeat an unchanged entire suite without cause.

### V2-051 — Release package and owner approval

- **Deliver:** concise release PR(s), changelog/footer change using `internal/web/changelog.go`, user help/compatibility/attribution, operations runbook, rollback steps, and summarized evidence.
- **Accept:** version proposed as 2.0; no second version constant; source pin and built resource provenance visible; deployment behavior and any game interruption explained. Backup verification is included if DB/schema changes require it.
- **Owner gate:** request approval of concrete PRs and release actions only after preparation. Record owner approval before merge. Main-branch auto-deploy must be treated as a deployment consequence, not an administrative merge detail.

### V2-052 — Approved deployment and smoke checks

- **Deliver:** owner-authorized rollout and final evidence/status update.
- **Accept:** deploy with CPU access controlled, verify health and existing flows, run a CPU smoke match, then enable only the approved access/capacity. Monitor the initial observation window defined in the approved runbook; if instability or existing-site regression occurs, disable CPU admission and follow rollback. Do not announce successful release before checks complete.
- **Check:** engine/build identity, five decks, custom setup, reconnect, cap, error handling, deck save/goldfish smoke, and release version. Mark release done only with deployed evidence and owner approval recorded.

## Required bug cards

### BUG-064A — Commander selection

- Reproduce [#64](https://github.com/quehorrifico/manatomb-v3/issues/64) for Surtr, Fiery Jötun and Kaalia of the Vast on the actual baseline.
- Identify data lookup, eligibility, name normalization, printing, or UI cause before editing. Do not hard-code the two names as a fix.
- Acceptance: both valid commanders can be selected through affected creation/edit paths, maintain correct card/printing identity through save/reload, and similar non-ASCII/case/printing scenarios regress correctly. Preserve existing invalid-selection handling.

### BUG-064B — Save frequency and truthful save state

- Reproduce draft/save behavior across signed-in and guest workbenches, slow/failed saves, reload and playtest return. Define an appropriate bounded debounce/flush policy based on existing persistence rather than arbitrarily increasing writes.
- Acceptance: UI distinguishes unsaved/local-only/saving/saved/error as applicable; an account-save success is shown only after acknowledgment; stale responses cannot overwrite newer edits; failure preserves recoverable draft data. Signed-out users are never told a deck is saved to their account.
- Check navigation/reload, authentication transition, request failure/retry, rapid edits, and playtest round-trip. Record actual persistence guarantees; do not promise that browser-close network requests always complete.

For either required bug card, an already-fixed baseline can satisfy the card with reproduction/regression evidence instead of a new change. A report that cannot be reproduced needs an explicit documented disposition accepted by the owner; it is not labeled “fixed” automatically.
