# Local release review checklist

Working branch: `codex/v2-release-candidate`, baseline `0ab74245912407e98f2cb5131a431a67cfed0f1f`, with preserved uncommitted changes. This records evidence and remaining work; it is not an approval to release or enable the feature. Historical M1 benchmark failures remain unchanged.

| Requirement | Current evidence | Result / remaining condition |
| --- | --- | --- |
| Actual human versus CPU, custom decks in both seats | `V2-M2/first-natural-game.json`, `custom-natural-game.json`; actual controls, manual mana and commander recast | Passed for MVP corpus |
| Exactly five inspectable CPU defaults | Five natural browser CPU results, upstream/deck hashes in `services/forge/default-catalog.json`; styles now in setup | CPU natural results passed; not tactical calibration or full human-seat coverage of every list |
| Required prompt bridge | Choice/ordering/privacy regressions; actual browser X, kicker, replacement order, targeting, stack responses, scry, combat and commander interactions | Existing mapped corpus passed; use `PROMPT_COVERAGE.md` for precise broader gaps |
| Two blockers and combat removal | `V2-M2/combat-tablet.json` | Passed; additional actual-browser 5/1/0 attacker assignment and resolution passed in `V2-local-qa.md` |
| Ownership, hidden state, stale/competing replies | Go race tests and actual pinned human-controller regressions; real packaged owner/retry/fault checks | Passed for documented boundaries; not a claim of exhaustive MTG information-flow proof |
| Reconnect/cancellation/reclamation | Ten packaged lifecycle cycles and all terminal game captures | Passed documented cycles; active CPU computation timing with queued concession is not measured |
| Concession | Actual direct priority and queued synchronous-choice browser paths; queued EDT hold regression | Passed. Queued intent waits for safe priority, cannot silently answer a choice; immediate End game is cancellation |
| Builder, saved CPU deck and goldfish integration | Saved/draft/goldfish real-browser return and immutable snapshot checks | Passed documented journeys; goldfish return begins a fresh hand, not a persisted goldfish game |
| P1 Surtr and saved-note recovery | `evidence/V2-bug-fixes.md`, actual PostgreSQL and browser failure/reload/retry | Fixes reviewable; owner release acceptance pending. Kaalia remains not reproduced |
| Theme/responsive/keyboard | `V2-M2/appearance.json`, `operations-ui-smoke.json`; all six account themes, three widths and actual keyboard controls | Targeted IAB checks passed. Full standalone Chromium, Firefox, desktop Safari/WebKit, physical tablet and screen-reader journeys unverified |
| Diagnostics | Explicit metadata allowlist, build identity and inspectable browser preview | Current file delivery passed twice, including a live reveal prompt; readable JSON exactly equals preview and metadata allowlist (`V2-local-qa/diagnostic-active.json`) |
| Separate capped packaged service | 2 GiB / one CPU / no swap, one global slot, Linux AMD64 under Rosetta; packaged Go web→engine | Passed local package integration. Native/provider capacity, cold storage and real catalog/site load unverified |
| Healthy existing-site traffic | `V2-M2/site-comparison.json`: 30 baseline + 30 idle-engine requests, no errors/429s, notes preserved | Passed bounded local idle comparison; not active CPU or production load proof |
| Drain, readiness, bounded retention and watchdog | `V2-M2/operator-smoke.json`, focused race tests, source caps and recorded worker/file cleanup | Passed local operator smoke. Provider rolling-deploy and routing checks owner-run |
| Final local checks/version/package | Full Go tests/race, vet, affected JS, CSS consistency, both Linux AMD64 images, final browser/operator smoke; `V2-M2/final-build.json` | Passed local integration; proposed version 2.0, uncommitted. This does not pass the separate hosted/browser release gates |
| G2 and hosted acceptance | Owner's local implementation authorization does not accept playthrough or hosted targets | Owner decision pending; no hosting/account access by agent |

Do not repeat natural games or the full lifecycle suite merely because a presentation-only file changed. Run the affected boundary once and preserve the old artifact's original identity. Missing ordinary gameplay input is an adapter defect; never reclassify it as an automatic compatibility exception.

## Decision at local handoff

**Code/build review: ready. Public release verification: blocked.** No known failing test in the exercised current local corpus is hidden by this distinction. Owner G2/P1 acceptance, unavailable-browser/physical-device checks and native/hosted performance/private-path validation remain unverified. V2-012 historical headroom/startup failures remain failures. No new evidence approves G1 hosted capacity or an infrastructure purchase.

The final smoke used normal Forge shuffle with Silvos/99 Forest in both seats; Keep advanced to actual CPU-turn human priority and cancellation reaped the worker. It is **not** another natural result or representative performance repetition. Whole-container smoke peak and cleanup are in `V2-M2/final-browser-smoke.json`; no heap-only or disk-size capacity conclusion is drawn.

The required owner actions, manual deck/playthrough instructions, deployment settings and rollback are in [REVIEW_HANDOFF.md](REVIEW_HANDOFF.md) and [OWNER_DEPLOYMENT.md](OWNER_DEPLOYMENT.md). Ordinary site features remain independently available; new database migrations: none.

## September 23 agent-owned verification closure

[Bounded QA report](evidence/V2-local-qa.md): all three specifically requested browser input paths passed actual Forge resolution; diagnostic delivery/privacy passed; crowded-board/long-catalog/keyboard checks passed in IAB. Large catalogs now use bounded explicit selection. Affected CPU JS tests: 8/8. Portable relocated-cache packaging and both images passed; fresh acquisition/build recipe is explicit, without repeating the Forge reactor build. Final packaged classes exactly match the browser-tested fixture overlay. Normal QA service liveness/readiness passed.

**Agent-owned local work completed.** Unavailable standalone browser/physical touch and reduced-motion preference checks have a short manual checklist in the report. Owner G2/P1 acceptance and separate owner-run hosted checks are not passed. Review instance 18880 was preserved; the corrected independently runnable build is on 18890.
