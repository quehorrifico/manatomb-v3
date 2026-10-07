# Desktop playtest controls — September 27–28, 2026

Owner-requested follow-up to the Forge-style table. Local implementation is reviewable; the owner-reported Raph & Mikey / Old Gnawbone combat crash has now been reproduced and corrected. No release acceptance, deployment, or live-service restart is implied.

## Changes

- The active desktop game occupies the viewport. Battlefield cards resize to their available lanes; the hand overlaps as it grows. Visible zone galleries also resize to fit. Life, floating mana, phase stops, zones, decisions, stack, and preview stay in the table. Long card text, activity history, and unusually large choice lists use contained scrolling; the document and battlefield do not scroll.
- Both seats show all six mana counts, including zero. Only the human mana offered by the current payment prompt is clickable. Auto-pay submits Forge's enabled Auto button using its existing prompt-bound OK action.
- Each seat has its own phase-stop column. These preferences control human priority during that seat's turn. Changes preserve forced choices and use Forge's own yield implementation. Untap is disabled because it has no priority; cleanup follows Forge's conditional priority rules. Acknowledged settings remain visible while the supervisor refreshes its cached state.
- Selected cards have an outline and checkmark. Attacks, planned blocks, and declared blocks have distinct arrows; blocker labels name their public attacker. The preview remains usable by hovering or keyboard focus.
- CPU launch is available from the deck editor, the home Playtest section, and the new Extras activity page reached from the header. The goldfish launcher was removed. Direct `/cpu` navigation still supports login, refresh, and returning to a session; this is a navigation change, not a referrer-based access restriction.
- A guide dialog and compact game-options menu keep secondary controls out of the main table. Failure references now identify a safe adapter operation and exception class instead of only `adapter_failure`.

## Current verification

- `node --test internal/web/assets/cpu.test.cjs internal/web/assets/cpu-launch.test.cjs`: 22 passed. Covers prompt safety, stale/rejected replies, privacy, floating/spendable mana, phase preferences including a stale cached view, selection labels, and Auto-pay action identity.
- `GOCACHE=/tmp/manatomb-go-cache go test ./internal/web ./cmd/server`: passed, including Extras method/path/rendering checks and canonical metadata. Temporary local test-server binding required sandbox escalation.
- `GOCACHE=/tmp/manatomb-go-cache go test ./internal/forge`: passed with the same temporary loopback permission.
- CSS parsing, JavaScript parsing, and `git diff --check`: passed.
- Real browser checks used an isolated fixture server on port 18765 and actual rendered ManaTomb templates/assets. Normal board and payment views were checked at laptop sizes. Crowded views with 52 permanents and a 16-card hand fit at 900×650, 1024×768, 1280×720, and 1440×900 without document or card clipping. A 48-card graveyard fit its gallery without scrolling. Normal view at 1366×768, CPU-turn phase toggling, preview focus, guide, menu, home launch, and Extras activities were also checked. These were presentation fixtures, not a new completed game or real-browser engine integration claim.
- `ControlsRegression` passed against the pinned Forge runtime: real automatic payment taps sources, both floating pools, forced-choice-safe preferences, empty-stack phase skipping, retained spell-stack responses, legal/illegal trample and blocker allocations, planned/declared combat links.
- Existing `WorkerRegression` passed against the changed worker: public projection, privacy/reveal, competing/stale replies, receipts, and combat constraints.

Engine evidence, compiled classes, source identities, and logs:

- `/tmp/manatomb-forge-controls-regression-final/`
- `/tmp/manatomb-forge-controls-worker-final/`

## Confirmed combat crash and action shortcuts

The owner identified Raph & Mikey, Troublemakers attacking together with Old Gnawbone, which entered tapped and attacking from the commander's trigger. A focused game using the real human controller reproduced `adapter_failure_askValues_UnsupportedOperationException`: after combat damage, removal of Forge's internal annotations made both Old Gnawbone trigger labels identical, and the adapter rejected their ordering choice.

The correction preserves only bounded, authorized damage-source, target, and amount context. Both entities must exactly match already-authorized projected entities. Hidden and unknown annotations remain excluded. Original Forge abilities and explicit order selection remain intact. The actual sequence now resolves both triggers in a chosen reverse order, creates 14 Treasures, and leaves CPU life at 26. Unequal damage amounts and hidden/unknown references were also checked. Evidence: `/tmp/manatomb-damage-trigger-shortcuts/`; the pre-fix failure remains at `/tmp/manatomb-damage-trigger-repro/regression.log`.

Following the owner's September 28 feedback, the adapter now matches Forge desktop's sole-action shortcut after an explicit card click and for the unchanged additional-cost continuation. Playing a land, tapping a basic land for mana, and casting Grizzly Bears reach the intended action/payment without redundant ability menus. Forge's `promptIfOnlyPossibleAbility` flag is respected; optional choices outside these paths and multiple actions still ask explicitly. Evidence: `/tmp/manatomb-action-shortcuts/`. The exact Gnawbone sequence passes with these shortcuts enabled.

The UI now shows shared trigger text once, with the distinct damage events in compact reorderable rows. It removes the duplicate five-phase strip and repeated battlefield headings, reduces panel borders, keeps all 26 phase controls, and initially previews the player's authorized commander/card. The full two-trigger decision and confirmation fit at 1024×768 and 1366×768 without scrolling. A supervisor transport check confirms phase updates retain the same forced-choice ID/revision, preserve an explicit false setting, enforce ownership, and deduplicate receipts.

A runtime check found the local Forge container still using build `cd54147f9ee47305fde24412ea218e44e262d6a8d32b253a1f7c6619a3896b26`, which predates the phase/Auto/crash fixes. It reported zero active workers. The owner requested the fixes continue; the new local image and web binary are ready. Automatic approval review rejected the service restart because earlier instructions reserved owner service restarts for approval. The owner has been asked to authorize loading these exact builds; no restart or drain was performed.

## Prepared runtime and loading the changes

The local `manatomb-forge:dev` image has been built successfully (manifest `sha256:32f5f2fc203dbecd0e030a49f7bdaf6273858732f91f0a6bf59475f9c9ed60a3`). A current local web binary is ready at `/tmp/manatomb-desktop-playtest-server`.

A fresh dedicated bundle was packaged using the existing verified engine cache; no Forge rules rebuild:

```sh
python3 services/forge/package.py \
  --cache /Users/zeusborrego/Developer/forge/v2-010 \
  --output /tmp/manatomb-desktop-playtest-bundle \
  --javac /Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home/bin/javac
```

Bundle identity SHA256: `6556ff0929ee8fb7b77ab75f16f9cbbf37af9278c8034f8855937bed80042c27`.

The running web/Forge services were left untouched. After ending any active game, the usual development workflow can load the new bundle:

```sh
docker build --platform linux/amd64 \
  --build-context forge-runtime=/tmp/manatomb-desktop-playtest-bundle \
  -f services/forge/Dockerfile -t manatomb-forge:dev .
docker compose --env-file .env -f services/forge/compose.dev.yaml up -d
```

Restart the Go web process with the usual `go run ./cmd/server` workflow. The bundle lives in temporary storage; regenerate it if that directory has been removed. Container update/restart ends any active Forge session.

## Source identities

| File | SHA256 |
| --- | --- |
| `internal/web/assets/cpu.js` | `6f442ac72cfb6360d22268ecdc4c35d3dacf4446c5c04ef57fe7d54e185dd788` |
| `internal/web/assets/cpu-table.js` | `764a622165f98d7fb77dbff8cdea65b9a8340084e46162014a4c14c87363ac40` |
| `internal/web/assets/cpu.css` | `b73cbe8a5ec89f02a4d94daa0cb5d2eaf1582ebd444e6fb1b1d6706c37fc5535` |
| `services/forge/src/ForgeWorker.java` | `8774d516f22549bc4c61884f5420abb28d43d201ca97382aa763cffa27db48bc` |

One primary and one bounded engine helper. No broad benchmark or full-game loop was repeated. Token/cost counters were unavailable.
