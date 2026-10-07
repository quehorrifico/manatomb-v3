# ManaTomb v2 — local review handoff

**September 23, 2026. Working local candidate; not approved for public enablement.** All work is preserved and uncommitted on `codex/v2-release-candidate`, based on `0ab74245912407e98f2cb5131a431a67cfed0f1f`. No push, merge, GitHub publication, hosting operation, infrastructure change or deployment occurred.

## What is ready to review

One signed-in human plays one Forge CPU in Commander through the real human controller. Both seats accept custom decks; five inspectable, versioned CPU defaults are included. Forge remains authoritative for legality, payments, choices, combat, triggers and outcomes. Saved/draft builder and goldfish entry points take snapshots; playing cannot overwrite the source deck. Partner/Background setup is supported without changing the editor's persisted single-commander model.

The separate authenticated engine supervisor admits **one game across the site**, returns busy when occupied, isolates each game in one JVM, and reaps it before releasing capacity. Hidden views, prompt identity, first-valid competing replies, bounded receipts/logs/history, reconnect, cancellation, expiry and failure states are server-controlled. Queued concession reaches the actual controller at safe priority; an outstanding synchronous choice still requires an explicit reply. End game immediately cancels without inventing a winner.

Both confirmed P1 fixes have actual PostgreSQL/browser evidence: commander-name normalization and recoverable saved-note failure. They remain in review. **Kaalia remains not reproduced; it is not labeled fixed or closed.** GitHub #64, #49 and #42 remain open and unchanged.

## Validation and identities

[Final checklist](RELEASE_CHECKLIST.md) distinguishes passes, failures and unverified conditions. [Integration report](evidence/V2-M2.md) links actual browser natural games, all five CPU defaults, focused complex choices/combat and ten packaged lifecycle cycles. [Prompt matrix](PROMPT_COVERAGE.md) separates controller coverage from actual browser play. Browser evidence is agent-operated real controls, not an owner's manual playthrough.

Final integration checks passed: full Go tests and race suite, vet, affected JS tests, CSS build/consistency, pinned adapter compilation/regression, both Linux AMD64 image builds, private operator smoke, and final packaged browser Keep → real priority → explicit cancellation/reclamation. Previous natural-game/fault results retain their original hashes; presentation-only changes did not trigger another entire suite.

[Final build manifest](evidence/V2-M2/final-build.json) records file hashes, image IDs, sizes and toolchains. These are local image IDs, **not published registry digests**. The runtime pin remains `26d8aff87509dde8a9d5017852339382d7ab3e83`; no Forge engine rebuild or card-rule patch was made. Bundle: `/Users/zeusborrego/Developer/forge/v2-release/bundle`. Exact logs and historical raw artifacts remain in the parent directory. Local Docker runs Linux AMD64 through Rosetta on ARM64; it does not establish native hosted performance.

## Try the local candidate

The updated independently runnable review build is at **http://127.0.0.1:18890/cpu** with normal Forge shuffling and no test fixture. The owner’s existing **18880/18881** review services/game were preserved and not restarted; that running web image predates the large-catalog selector correction. QA uses a separate disposable database and game slot. The intentionally disposable account is `v2-local@example.invalid`, password `Local-v2-test-only-2026`. These are local fixture credentials, never hosting credentials.

1. Sign in and open CPU play. Inspect a default deck. For a realistic human deck, paste your own 100-card Commander list, or copy an inspected default's commander/main sections into the human seat. The synthetic saved “V2 local recovery fixture” is 99 Forest plus Silvos and is only a boundary fixture.
2. Choose a different default or custom CPU deck and start. Make play/draw and keep/mulligan decisions. Click cards and read the actual Forge choices; mana/payment is explicit. Use Pass/OK or End Turn only when offered.
3. Cast and respond, attack/block, and exercise commander movement/recast. Ordered prompts explain whether position one resolves first or is the first returned item. Do not infer resolution order from generic list position.
4. Refresh at a decision: the live game should return. A second tab's stale control must resynchronize instead of applying twice. An occupied global slot must show busy to another account.
5. Finish a natural game for owner acceptance; separately try Concede versus End game. The former uses Forge's result, the latter is irreversible cancellation. Confirm a new game can start afterward.
6. Check the sanitized diagnostic preview and downloaded JSON. Actual JSON file delivery now passed twice, including during a live hand-reveal prompt; its contents exactly equal the sanitized preview. Check saved-note failure/recovery and Surtr if reviewing the P1 changes; exact reproductions are in [the bug report](evidence/V2-bug-fixes.md).

For a clean local installation, use [the service README](../../services/forge/README.md) and its Compose file. Rebuild commands and pinned runtime assembly are there. `tools/forge-service-validation/restart-local.py --env-dir /Users/zeusborrego/Developer/forge/v2-release --web` reuses this existing disposable topology and refuses to interrupt an active JVM. It does not initialize a new machine. Local Compose defaults CPU to disabled; explicitly set `CPU_PLAY_ENABLED=true` only in the local environment file when testing.

## Filled MVP acceptance record (G2 still pending)

| Check | Local result |
| --- | --- |
| Owner implementation/hosting direction | Conditional local G1 accepted; 2 GiB sizing is an assumption, not hosted acceptance |
| Signed-in saved/pasted human and custom/default CPU | Passed browser setup/snapshots and Forge validation |
| Mapping errors and commander pairing | Passed parser/controller conformance, unknown-card errors and Partner/Background fixtures |
| Two natural games, custom and default opponent | Passed, exact separate records in V2-M2 |
| Decisions, costs, stack, combat and commander flow | Passed documented browser corpus |
| Identity, hidden state and lost acknowledgment | Passed protocol/privacy/race and packaged lifecycle evidence |
| Reconnect, engine death, cap and reclamation | Passed ten packaged cycles plus controlled expiration and final smoke |
| Existing builder/save/goldfish behavior | Passed documented browser/P1 round trips |
| Owner playthrough and acceptance | **Pending**; no owner approval inferred from agent tests |

## Remaining release conditions

- **Owner acceptance:** record G2 playthrough and P1/disposition review. Implementation authorization was not acceptance of every release gate.
- **Browser/device matrix:** unavailable full standalone desktop Chromium, Firefox, Safari/WebKit and physical tablet-touch journeys, plus OS reduced-motion preference check. Crowded-board/long-catalog and keyboard interaction now passed locally. Current evidence covers the connected in-app browser, six themes, keyboard controls and responsive widths, not every engine/device. The Chrome automation provider was unavailable; no browser installation/setup loop was started. Current diagnostic file delivery is verified; no owner repeat is required merely to close that gap.
- **Coverage limits:** generic amount splitting, searchable card-name choice and manual attacker damage across multiple CPU blockers now passed focused real-browser/Forge resolution; token/copy/current transformed face and hidden metadata now have engine-backed projection regressions. Five defaults reached natural results as CPU; this is not complete human-seat coverage or AI-strength calibration. No known ordinary missing prompt was silently accepted as a compatibility exception.
- **Hosted resource acceptance:** native 2 GiB/one shared CPU startup/headroom, cold-cache behavior, provider routing/reconnect/drain, and healthy realistic site traffic during active gameplay remain owner-run. Historical 1 GiB headroom failures and missed 30-second startup samples remain failures. The final local smoke does not replace five matched resource samples. Actual CPU-computation concession latency is not measured; deterministic blocked-event-thread and synchronous-choice paths were verified.

These conditions keep **V2-050 public release verification blocked**. The application/build deliverable is reviewable; the roadmap has not been silently relaxed or declared complete. Agent-owned local verification is complete. The remaining steps are the short owner/device checklist and owner-operated hosted acceptance; no generic investigation phase is proposed. Any new concrete adapter failure requires a focused reproduction/fix, not automatic answers or a new engine rules layer.

## Owner deployment actions after review

Follow [OWNER_DEPLOYMENT.md](OWNER_DEPLOYMENT.md): keep the existing web and independent managed PostgreSQL, add one private App Platform **service** for Forge at 2 GiB/one shared CPU, approximately **+$25/month** (planning total $45.15 before extras). Capacity starts at one global game. This is a sizing assumption, not an observed production guarantee.

The owner publishes the reviewed Linux AMD64 images with corresponding source/notices, records registry digests, configures internal port 8081, matching runtime secret and web URL, verifies liveness/private routing, then deploys the web with CPU **disabled**. No database migration is added. Run acceptance in a controlled test environment before deliberate public enablement; the flag enables all signed-in accounts on that web instance, not an allowlist. Keep a matched-image rollback, drain before replacing the engine, and observe at least 30 minutes plus one completed smoke game. The guide specifies exact variables, health checks, order, stop conditions and rollback.

No hosting purchase or merge approval is requested as part of this local handoff.

## Latest local correction and shortest remaining checklist

[September 23 verification](evidence/V2-local-qa.md) closes the three requested browser paths and actual diagnostic download. One UI defect was corrected: 33,640-name prompts now render at most 60 searchable offered choices, preserving explicit selection and original Forge IDs. Fresh runtime acquisition/packaging instructions are in the service README; relocated-cache packaging and both Linux AMD64 images passed. [Current build identities](evidence/V2-local-qa/build.json) supersede the earlier final-build manifest only for these new files/images; previous natural/lifecycle evidence remains intact.

1. **Owner review:** finish your natural game/playthrough and record G2/P1 acceptance. You can keep playing on 18880; use 18890 for the corrected selector when ready. Kaalia is not claimed fixed.
2. **Unavailable devices:** one short play/resume/choice/refresh/end check in your available standalone browsers and on a physical tablet (both orientations), with keyboard and OS reduced motion. See the exact short checklist in the QA report.
3. **Hosting, separately:** follow OWNER_DEPLOYMENT.md to publish/deploy the reviewed images yourself, configure the private separate Forge service, run hosted smoke/resource checks, then deliberately enable CPU play. Hosting has not been validated or operated by the agent.
