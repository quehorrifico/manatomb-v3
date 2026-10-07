# Implemented v1 contract checkpoint

The September 18 implementation supersedes the proposed names/statuses below. See [V2-020 acceptance and examples](evidence/V2-020.md) and `internal/forge/contract.go`. The prior draft is retained as historical research, not a claim that all proposed features were implemented. Current receipts record admission/HTTP rejection, not separate applied/resolved events. Cancellation is process termination, not gameplay concession. The deployment and broad prompt/UI gates remain pending.

The current service also exposes private secret-authenticated operator drain/status routes, never proxied to browsers. Admission closes before shutdown cleanup. `FORGE_NO_PROMPT_TIMEOUT` defaults to 90 seconds for a continuous working interval or unresponsive worker; human prompt waiting uses the separate idle/grace limits.

`requestConcede` is an explicit **session termination intent**, with `request`, engine `session`, `revision:0`, and `prompt:""`. It is not a prompt reply and cannot carry selections/amounts. It becomes available only after opening decisions. HTTP sets a queued flag, never mutates Forge objects. At the next stable `InputPassPriority` on the event thread, the adapter invokes the real human controller's `concede`. While queued, required synchronous choices still require an explicit human reply; cancellation remains immediate. A request is not a finished concession, and Forge's result wins if the game finishes first. Regular `concede` retains prompt/revision validation at an already offered priority control. Retired incarnations and unavailable requests are rejected, and receipts remain bounded/idempotent.

# First session contract — v0.1-draft (V2-013)

**Proposed, not frozen, implemented or approved for production.** G1 remains pending; V2-012 remains blocked. This is a decision artifact, not V2-020/M2 implementation. The [decision report](evidence/V2-013.md) defines the conditions. The exact Forge code/resource/config identity is mandatory; current proof uses `26d8aff87509dde8a9d5017852339382d7ab3e83` and the manifests linked in V2-010–012.

## Authority and ownership

Forge alone executes rules, legality, turns, priority, payments, targets, stack, combat, randomness and CPU choices. Go authenticates and authorizes the account, snapshots deck inputs, reserves capacity and owns termination. The adapter translates Forge's offered input and permitted view. The browser displays that view and submits intentional choices. No raw Forge objects, card-rule inference from text, client-authoritative life totals, default answers to unsupported prompts, or AI replacing the human.

Proven: one human, one CPU, one game per process, seed/profile/decks/config recorded. Proposed: one active match per account and global cap one initially, enforced atomically before spawning. Concurrent admission across replicas, account authorization, restart reconciliation and production resource limits are unimplemented. Do not translate two registered users into capacity for two games.

## Transport and session creation

Propose same-origin JSON commands plus a single state-poll transport for the first contract, grounded in the current GET `/state` and POST `/action` spike. Production URLs below are proposals. Poll cadence/backoff, unchanged-revision responses and traffic budget must be validated; the spike's 200 ms full-view polling is **not** an accepted production cadence. Do not add WebSocket and event-stream implementations in parallel. Actual provider ingress/timeout behavior remains unverified.

`POST /api/cpu/sessions` consumes an authenticated owner's immutable human/opponent deck snapshots. Use names, counts and complete commander section(s); user-supplied scripts/executables are prohibited. Resolve to the pinned Forge catalog, retain canonical deck hashes, and return actionable missing/ambiguous/deck-conformance errors without silently replacing cards. Multi-commander representation, import normalization and builder integration remain V2-020 work; the existing product has a single commander field.

Proposed successful creation (202 means reserved/starting, not ready):

```json
{
  "protocol": "manatomb.cpu/0.1-draft",
  "sessionId": "opaque-new-incarnation",
  "status": "starting",
  "humanSeat": 0,
  "engineIdentity": {
    "forgeRevision": "26d8aff87509dde8a9d5017852339382d7ab3e83",
    "adapterHash": "sha256",
    "resourcesHash": "sha256",
    "configurationHash": "sha256"
  },
  "deckHashes": {"human": "sha256", "cpu": "sha256"}
}
```

One opaque session ID represents one incarnation; never reuse it for replacement workers. A separate internal worker/process identifier is not exposed. Cap exhaustion returns an honest busy response without queuing a hidden game or starting a worker. Proposed HTTP 409 `capacity_busy`; production error taxonomy is not implemented. Do not transmit DB credentials to Forge. A private engine service accepts authenticated Go requests only; the local spike's loopback token is not production account authorization.

## Human-specific view

`GET /api/cpu/sessions/{sessionId}` authenticates the owner and chooses the human viewer server-side. No client-selectable CPU/spectator viewer. The spike demonstrates a scalar JSON DTO: status, session incarnation, monotonically changing revision, turn/phase/player, players/zones/cards, public stack source and current prompt. Visible cards carry display fields and session-scoped entity IDs; concealed zones provide permitted counts without identities or hidden IDs.

Preserve Forge-authorized reveals, including scoped CPU-hand reveals and search choices. Revoke visibility when the prompt/reveal scope ends. Do not expose hidden ordering, unrevealed identities, raw stack/ability metadata, private logs or text with hidden names. Sort visible search choices without disclosing original library order. Do not rely on CSS to hide private data. Existing fixture tests prove selected hidden/revealed boundaries; face-down, transformed/copy and broad custom-card coverage remains incomplete.

A prompt contains `promptId`, `kind`, constraints, authorized option/entity IDs, labels/context and enabled actions. The spike calls these `id`, `min`, `max`, `options`, `okEnabled`, `cancelEnabled`; proposed production naming needs an explicit translation. IDs are opaque to the browser and valid only for that session/prompt; do not derive legal actions by parsing card text.

Known demonstrated kinds: keep/mulligan, priority/yield, card/player input, spell/ability choice, manual mana/payment, confirm, entity/target/search choice, single-card scry location, attack/block, commander replacement and the narrowly supported simultaneous-ability permutation. Unsupported prompt families terminate visibly with an opaque diagnostic ID; no automatic pass/first option. General ordering, arbitrary damage assignment and broad special cases remain unimplemented.

For simultaneous abilities, every offered option appears exactly once. Browser positions describe resolution order among those abilities, subject to later responses; Forge places the returned original view list in reverse. Remember is disabled. Missing, duplicate, extra, malformed or stale IDs are rejected before consuming the prompt.

## Command/reply envelope and acknowledgment

Proposed envelope retains the proven session/revision/prompt/request checks:

```json
{
  "requestId": "unique-client-command-id",
  "expectedRevision": 226,
  "promptId": "opaque-current-prompt",
  "action": "reply",
  "selectedOptionIds": ["option-B", "option-A"]
}
```

The authenticated URL binds the session; include/validate its incarnation in internal messages. Mutation requests require CSRF/origin protections and size limits. The spike's equivalent fields are `request`, `session`, `revision`, `prompt`, `action`, `selected`, and a local CSRF token. Entity selection instead sends an offered entity ID. Empty selection is allowed only when Forge's prompt explicitly permits it. UI disablement is convenience, not server validation.

Proven: stale/repeated/malformed/competing replies are rejected (409 in the spike); exactly one racing reply consumes the pending prompt. A 200 response means **admission**, not successful spell resolution. Forge may reject a legal-looking click afterward and publish an action error. No result is fabricated.

Proposed production retry behavior differs from the spike: retain a bounded per-session command receipt indexed by request ID and payload hash. Same ID + identical payload returns the known receipt; same ID + different payload conflicts. Receipt states distinguish admitted/applied/rejected-by-engine. This ledger, retention, lost-ACK recovery and request lookup are **unimplemented**. On uncertain receipt, fetch/resync current state rather than sending a new command blindly. Worker death ends the session; no crash-proof exactly-once claim or automatic game replay.

## Serialization and blocking

Proven adapter mechanism: synchronized pending-input gate and revision checks; real input mutations dispatch onto the Swing event thread with an in-flight action guard and actual Forge input identity validation. Synchronous human choices may block that same thread on a future. A separate HTTP reply path validates/consumes the current prompt and completes its future directly, allowing the engine to unblock. Never enqueue the needed reply behind the action that is waiting for it. The regression exercises a blocked AWT thread and competing reply threads.

Do not assume `GameAction.invoke` is a per-game serial executor. Production controller ownership, visibility snapshots and lock boundaries must preserve the proven mechanism and undergo concurrent-input tests; do not access mutable Forge objects from arbitrary request threads. One game per process contains Forge globals; no multiple-game JVM concurrency assumption.

## Lifecycle and terminal states

Proposed public states: starting → waiting-for-human / engine-working → finished or terminated. The spike has `starting`, `input`, `choice`, `working`, `finished`, `blocked`; these map explicitly, not by claiming all production semantics are already implemented.

- **Finished:** winner/outcome from Forge. Focused natural human results and AI natural results are demonstrated separately from the turn-12 benchmark aborts.
- **Conceded:** only after a real Forge concede path yields a result; production browser concession is not proved by killing the process.
- **Terminated:** reason `idle_expired`, `disconnect_expired`, `session_deadline`, `engine_error`, `worker_lost` or `administrative_stop`. Infrastructure termination is not a gameplay loss.

Propose idempotent `DELETE /api/cpu/sessions/{sessionId}` for termination, with owner authorization. Cancel outstanding input and bound worker shutdown, escalating to process kill when needed. Release capacity only after worker reaping is confirmed; duplicate cleanup must not release another session's reservation. Record a small terminal tombstone so stale controls cannot address a replacement. Crash reconciliation/tombstones are production work; local supervisor cancellation, escalation, worker death and replacement rejection are demonstrated.

Reconnect fetches the authorized current snapshot from the same live incarnation; it does not restore a lost worker. No durable Forge saves. Deployment/restart honestly terminates the session and preserves deck inputs for a new game. Proposed defaults from the technical plan remain **unvalidated**: five-minute disconnected grace, fifteen-minute waiting-input idle timeout, two-hour session maximum, warning before expiry. Browser heartbeats do not renew human activity. A CPU watchdog threshold must be validated against demanding native workloads; subsecond sampled method calls are not a universal whole-turn deadline.

## Resource, diagnostics and acceptance

No production memory size is approved. Whole-worker/service accounting includes JVM native memory, supervision, caches, logs and transient control processes. Diagnostics must be bounded by count/bytes/retention and sanitized for the viewer. The spike's retained transcript and benchmark logging are evidence tools, not an implemented production retention policy. Keep privileged engine diagnostics separate; return opaque errors to the browser.

Before freezing: native matched-duration repeat measurements; healthy nonproduction site comparison; selected ingress/private routing and termination checks; owner-bound authorization and duplicate-receipt design review; broader visibility/prompt tests needed for chosen MVP corpus; exact cost/global cap and G1 owner decision. No G1 approval or M2 implementation is created by this document.
