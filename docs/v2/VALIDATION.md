# v2 acceptance and release evidence

> **September 18 owner update:** G1 is conditionally accepted for local implementation through the full release candidate. Historical benchmark failures stay recorded; separate 2 GiB/one shared CPU Forge is the planning target. The owner operates all hosting, with no agent account/remote access. Local work continues through G2/M3 preparation; owner playthrough, hosted acceptance and enablement remain separate pending checklists. No merge/push/deploy/purchase is authorized. See STATUS.md.

All checkboxes start unchecked. The planning document itself passes none of these gates. Record results against the exact app/adapter/Forge/resource/configuration pins in the corresponding task report, and link it from STATUS.md.

## Internal MVP gate — V2-031

- [ ] G1 feasibility and hosting direction accepted by owner; selected local/test topology reproduces the evidence.
- [ ] Signed-in tester can start with a saved or pasted custom player deck, and with a pasted custom CPU deck or provisional default.
- [ ] Unknown/ambiguous/unsupported cards and engine deck validation errors appear before play; no entries silently dropped or replaced.
- [ ] At least two complete human-vs-CPU games reach a natural Forge result: custom opponent and default opponent. Concede also works but is not the only game end tested.
- [ ] Opening decisions, priority/stack response, costs/targets/choices, combat, commander movement/recast and game end occur through the real bridge.
- [ ] Illegal/stale/duplicate input is handled by the server/engine contract; lost acknowledgments do not cause duplicate actions.
- [ ] Unrevealed CPU hand/library identities, hidden ordering and other unauthorized data are absent from browser payloads/logs, including errors; legitimate Forge-authorized reveals remain visible.
- [ ] Refresh and short disconnection recover the same live match; engine death produces an honest ended state and frees capacity.
- [ ] One account/global cap, idle expiry, cancellation and worker cleanup function; normal ManaTomb flows remain usable.
- [ ] Goldfish mode and existing deck save/return flows retain baseline behavior.
- [ ] Owner playthrough and concrete acceptance recorded. Limitations listed; M3 has not been substituted for uncompleted MVP work.

## Gameplay interaction corpus

Use pinned fixtures with known goals. The purpose is to verify ManaTomb carries Forge decisions/views faithfully, not to recreate the entire MTG rules test suite. Read the pinned human interfaces to extend this matrix; checking only five convenient decks is insufficient.

| Family | Minimum exercised behaviors | Evidence field |
| --- | --- | --- |
| Commander/setup | Single commander, Forge-supported partner/background arrangement, command-zone choices, tax/recast, commander damage, preset/mulligan/turn-order behavior | Fixture + source mode + result |
| Priority/stack | Pass/respond, multiple stack entries, instant in opponent turn, triggers/order selection | Prompt/action transcript |
| Costs/mana | Automatic/manual mana behavior exposed by Forge, alternate/additional costs, X, variable/colored requirements, cancellation | Input constraints + result |
| Targets/choices | Card/player targets, one/many choices, numbers, modes, yes/no, ordered lists, selections across zones | Prompt-family map |
| Combat | Attackers/blockers, multiple blockers, damage assignment, combat tricks, removal during combat | Browser scenario |
| Zones/visibility | Hand/graveyard/exile/command, search/reveal/reorder, face-down objects, temporary visibility, shuffle | Allowed-view assertions |
| Board complexity | Tokens, counters, attachments, copies, transformed/double-faced cards, many permanents | Fixture + readable render |
| Effects | Replacement choices, optional/mandatory triggers, discard/sacrifice, selection/order interactions | Prompt + engine result |
| Outcomes/failures | Natural win/loss, concede, unavailable prompt, engine exception, stuck computation, disconnect/expiry | Distinct terminal states |

Mark every reachable prompt family as exercised, supported but not yet exercised, or unsupported with explanation. “Unsupported” for an ordinary required interaction is a release blocker, not an automatic permission to narrow custom decks. A card-specific upstream defect is triaged in BUGS.md. Explicit owner-approved compatibility exceptions must be visible to the player and must not silently change rules.

## Public v2 product gate — V2-050

- [ ] All MVP checks remain valid on the release candidate.
- [ ] Exactly five defaults have list provenance, descriptions, validation, tested engine pin and browser match evidence.
- [ ] Player and CPU custom lists, player saved/draft deck and CPU owned-saved-deck selection work.
- [ ] Match snapshots protect saved/draft data; rematch, return and save-state behavior pass with unsaved edits and authentication transitions.
- [ ] Full interaction corpus and prompt inventory reviewed; no unaccepted required adapter gaps.
- [ ] Both themes and any additional existing appearance options render readable gameplay controls.
- [ ] Desktop/tablet setup and play, keyboard focus, touch alternatives, card inspection, stack/log, and error/slow-turn feedback work.
- [ ] Phone setup/help communicates the supported gameplay viewport without hiding inaccessible actions behind a false parity claim.
- [ ] Compact outcome and sanitized diagnostic export work; copy/help explains engine version and how to report a problem.
- [ ] BUG-064A/B and other required bugs have reproduced-or-already-fixed evidence and relevant regressions. Optional items have explicit dispositions.
- [ ] No unresolved critical data-loss/security/hidden-information issues, repeated ordinary-match failures or unreclaimable capacity leaks.

### Browser/device evidence

Freeze actual tested versions with the release candidate; don't pin future browser version numbers in the roadmap. Use desktop Chromium and Firefox, Safari/WebKit, and tablet touch coverage. Proposed viewports: 1440×900 desktop, 1024×768 tablet landscape, 768×1024 tablet portrait. Exercise a phone-width setup/help view (around 390 px), without claiming a full phone battlefield.

Test one full journey per browser family, and target platform-sensitive interactions (touch, drag alternatives, hover inspection, focus, long-lived connections) across the matrix. Record any unavailable physical device and distinguish browser emulation from hardware evidence. Include a crowded board, large hand, long prompt, slow engine, network loss and reduced motion.

## Operational and security gate

- [ ] Selected engine container limit and global cap come from measured candidate results; final paid topology/cost accepted by owner.
- [ ] A request above the cap returns busy without allocating another worker; concurrent starts/aborts cannot leak or double-book slots.
- [ ] Cross-account session read/action/reconnect/end denied; private deck authorization checked independently of deck ID possession.
- [ ] CSRF/origin rules, payload/rate limits, engine authentication and private routing verified on the actual deployment path.
- [ ] Browser responses, telemetry and exported logs contain only permitted information; engine errors do not expose secrets or hidden game objects.
- [ ] Engine OOM, hang, crash, readiness failure and restart tested; ordinary Go service functionality remains available and every occupied slot becomes reclaimable.
- [ ] Grace, idle and lifetime expiration tested with fake/controlled clocks or bounded test configuration; UI warnings reflect server state.
- [ ] Engine decision watchdog is based on observations; timeout ends the session visibly without gameplay substitutions.
- [ ] Build reproducible from pinned source/resources, no runtime dependency on a graphical display, card-art download, or persistent local disk.
- [ ] Log/temp/metadata retention caps and cleanup tested; actual DB disk usage and projected retained volume recorded.
- [ ] Deployment drain/termination, CPU kill switch and rollback rehearsed; existing session expectations documented.

### Load and soak evidence

Run one realistic Commander match continuously for at least 60 minutes or until natural completion, and at least ten sequential sessions including ordinary completions and aborted/disconnected cases. These may share the same fixture runs as other checks when their evidence satisfies both. Include each default and at least two additional custom decks chosen to exercise more demanding board/prompt patterns.

At the configured global cap, measure cold/warm startup, total memory, CPU decision latency, command delivery, threads, bytes transmitted, and cleanup. Attempt cap+1 starts and concurrent requests. Compare the existing app's search/deck/save workload with and without the engine. Record sample counts and outliers, and verify the budgets accepted at G1. If cap is increased later, rerun tests at that cap. An automated AI-vs-AI soak can supplement but does not replace browser human-input/reconnect tests.

## Existing project checks

At integration/release boundaries, follow the repository's current required checks, re-reading them if changed. At the inspected baseline these include:

```sh
npm ci
npm run build:css
git diff --exit-code -- internal/web/assets/tailwind.css
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/server
docker build .
```

CI currently runs the CSS consistency check, ordinary Go tests, vet and server build; the root README additionally calls for race tests and Docker build before release. Add targeted Java adapter and protocol/browser integration checks when those exist; record their exact runnable commands in V2-010/021. Avoid pretending those commands or tests already exist in this planning-only change.

Run focused tests during implementation, then broaden at the relevant boundary. When fixtures detect a failure, reproduce once, repair the cause, and rerun the affected set. Do not keep running successful unchanged checks to consume time. Document unrun checks and environmental failures separately from product failures.

## Release / rollback gate — V2-051 and V2-052

Before requesting merge/deploy approval:

- [ ] All required pre-release dependencies through V2-050 are in `review` or `done` with evidence; V2-051's release package is reviewable; no hidden blocked dependency. V2-052 remains pending until approved rollout.
- [ ] Owner-facing PR identifies task IDs, scope, validation, material limitations and exact deployment consequence.
- [ ] Public version/changelog/footer agree using the single existing version source.
- [ ] User help states supported mode/devices, capacity/busy behavior, session limits, transient reconnect versus restart loss, and Forge compatibility/attribution.
- [ ] Chosen license/notices/source-handling steps for actual packaged components are implemented.
- [ ] Database backup verified when schema/data changes require it; migrations preserve v1/rollback compatibility.
- [ ] Approved runbook specifies engine/app versions, config, deployment order, smoke steps, initial observation window and rollback triggers. Proposed initial observation window: 30 minutes plus completion of a CPU smoke game; extend only for concrete concerns.

After the prepared package is reviewed, before executing a merge or rollout:

- [ ] Owner explicitly approves the concrete merges and relevant paid/deployment actions; record the decision in STATUS.md. No auto-merge. This is not a prerequisite to asking for approval.

Rollout sequence: keep CPU admission disabled → deploy compatible engine/app artifacts → check readiness and ordinary flows → run allowlisted CPU smoke/reconnect/cap checks → enable the accepted access/capacity → observe the approved window → record final results.

Rollback triggers include existing-site regression, hidden-state leak, repeated start/ordinary-match failure, memory-limit breach, or capacity that cannot be reclaimed. Disable new CPU starts first; drain where safe or terminate with an honest message; restore the approved prior app/engine pair as needed. Existing goldfish/deck functionality must remain accessible. Do not delete user decks or reverse data changes blindly.

Release complete means approved code is deployed, the smoke/observation checks passed, required bug dispositions are recorded, and STATUS.md links the final evidence. A merged PR or enabled feature flag alone is not completion.
