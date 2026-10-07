# Bounded local verification — September 23, 2026

**Agent-owned local checks completed; owner acceptance, unavailable-device checks and hosted validation remain pending.** One primary agent, preserved uncommitted branch `codex/v2-release-candidate` / base `0ab74245912407e98f2cb5131a431a67cfed0f1f`. No push, merge, publication, deployment or hosting operation. The owner's running review instance at 18880/18881 was not restarted or reclaimed.

## Isolation and identities

Separate `manatomb-v2-qa` network, disposable `manatomb-qa-db` database, web 18890 and engine 18891. Engine: Linux AMD64 under Rosetta on ARM64 macOS, 2 GiB, one CPU, no swap, read-only root, 128 MiB tmpfs, one global slot. This is local functional verification, not a new capacity measurement. Private environment files remain outside the repository; no secrets are in this evidence.

[Build manifest](V2-local-qa/build.json) records current source/image hashes. The browser fixture used runtime identity `19c09554f90ec45a0c8b73f06100ceced871e75a3270b9c830b30f00448db0e1` with a read-only corrected fixture class overlay. All five overlay class files were byte-compared with the final portable package and match. The final package identity is `cd54147f9ee47305fde24412ea218e44e262d6a8d32b253a1f7c6619a3896b26`. Do not identify the earlier image alone as the corrected fixture.

Final images: `manatomb-web:v2-qa` and `manatomb-forge:v2-verified`. Both Linux AMD64 builds passed. QA now runs those files with **normal Forge shuffle, no overlay and no fixture**. Liveness/readiness passed. The owner's existing 18880 build remains running unchanged; the updated review build is independently available at **http://127.0.0.1:18890/cpu**, using the same disposable local account described in REVIEW_HANDOFF. It has a separate database and game slot.

## Actual diagnostic file delivery — passed

Clicked the real download control twice. A browser download-event wait timed out, but the first 411-byte JSON file existed in Downloads and exactly equaled its preview: [record](V2-local-qa/diagnostic-download.json). This was an observation limitation, not a reproduced download defect.

Repeated during a live turn-2 `InputConfirm` hand-reveal prompt, with human private cards and legitimate CPU revealed cards present. The newly delivered 421-byte JSON parsed successfully and exactly matched the preview: [active record](V2-local-qa/diagnostic-active.json). Strict field equality verified only format/protocol/pin/build, opaque session reference, status/turn/phase/prompt kind, failure code/result/conceded. No hand identities, deck lists, prompt prose, private text, credentials, raw errors, logs or internal diagnostics. The existing poison-data unit regression also passed. Download code did not need changing.

## Real browser inputs and actual Forge resolution — passed

These are agent-operated **browser controls through the packaged Go proxy and actual Forge human controller**, not controller-only substitutions. Setup is a focused fixture, not a natural game or realistic performance repetition. No rules were added and no prompt was auto-answered.

| Requested path | Explicit browser action | Observed engine result |
| --- | --- | --- |
| Generic multi-target amounts | Rolling Thunder, X=3; target CPU and Savannah Lions; enter CPU=1, Lions=2; manually pay five Mountain mana; pass priority | Stack described both amounts; CPU life 5→4; Lions moved to graveyard; the two 3/5 blockers survived |
| Text/card-name selection | Cast Cabal Therapy targeting CPU; pay Swamp; type `Vampire Noble` in the actual 33,640-option offered catalog; Tab/Space selected the radio and Tab/Enter submitted | Forge revealed CPU's hand, including Vampire Noble. After explicit OK, that card moved to graveyard and hand count fell 6→5. Remaining CPU hand identities were hidden again |
| Manual attacker damage to multiple CPU blockers | Attack with Colossal Dreadmaw under Goblin War Drums; actual CPU chose Siege Mastodon and Thraben Purebloods as blockers. Enter Purebloods=5, Mastodon=1, CPU=0 and submit | Purebloods died, Mastodon retained one marked damage, Dreadmaw died to combined blocker damage, CPU stayed at 5 life |

The allocation game (`1646a64a-532e-45eb-9224-32fe00d851b6`) expired during an interrupted naming check and was reclaimed. Naming/combat were completed in replacement game `54ceb6aa-e79c-462a-a811-97641d65491c`; prior allocation evidence was reused rather than repeated. Explicit End game/confirmation then reclaimed that fixture: no Java process and no session directories remained before switching QA to the normal launcher. No claim of natural completion is made for either fixture; previous natural-game evidence remains unchanged.

### Concrete UI correction

The original catalog prompt attempted to render all 33,640 radio controls and the IAB stalled/crashed. `cpu.js` now filters large single-choice lists by the offered labels and renders at most 60 matches. The player must explicitly select and submit; the original offered ID is preserved. A fabricated or unmatched free-text name cannot become a reply. Multi-choice and engine visibility/identity handling are unchanged.

Focused JS regression builds 33,640 options, verifies the render bound, exact original-ID submission, no auto-selection, and empty unmatched results. **8/8 CPU JS tests passed**, including private diagnostic exclusion, stale-error display, focus recovery, authorization expiry and bounded public activity. Actual card-name browser resolution then passed as above. No Forge rebuild or engine/card-rule modification.

Browser tooling also retained an old crashed-page result for the Playwright read path after a fresh tab was healthy; documented accessibility controls continued successfully. This later tool error is not claimed as a second product crash. No repeated browser setup/reinstall exercise.

## Interaction coverage and exact remaining device checks

- Connected IAB: actual sign-in/setup/keep, targeting/payment, name search via keyboard, explicit numerical inputs, prompt submission and cancellation passed. Existing natural/core-journey evidence is reused.
- Crowded actual board: 39 human permanents initially, two seat columns, wrapping labels and independently scrollable sticky prompt inspected visually; name search reduced the long catalog to one selected result. No observed overlap preventing controls. Previous responsive-width and six-theme evidence is retained.
- CPU styles introduce no animation, transition or smooth scrolling. **Reduced-motion OS preference switching is unverified**: the available browser backend has no advertised media emulation; no global owner setting was changed.
- Prior tablet-width checks are **responsive emulation with pointer/keyboard**, not physical touch or a physical tablet. This pass did not change the browser-wide viewport while the owner was reviewing another tab. Physical touch remains owner-run.
- Standalone Chromium/Firefox providers were unavailable. Safari localhost sign-in was observed, but native window routing was unreliable; no full Safari journey is claimed. Its initial automatic snapshot surfaced a pre-existing hosting tab; no hosting navigation/action was performed, and that app-control path was abandoned. No account information was used.

Short owner device check: in available Safari/Firefox/Chromium and a tablet, start/resume a game, scroll a crowded board/long prompt, select and submit a choice, use Tab/Space/Enter on desktop, refresh, then End game. On the tablet use actual touch and check both orientations. With OS reduced motion enabled, check prompt/board updates remain readable without unexpected movement. Record unavailable devices rather than inventing passes.

## Reproducible acquisition and packaging — bounded check passed

The [service README](../../../services/forge/README.md) now gives an explicit fresh-machine path: Python 3.12+, pinned Temurin 21.0.6+7, public pinned Forge/Maven archive URLs and SHA-256 verification, one reactor build into a private chosen cache, exact preferences/decks, adapter packaging, and both Linux AMD64 image commands. `prepare-runtime.py --plan` and Python syntax checks passed. The pinned runtime image's JDK was also checked as 21.0.6.

The packager previously consumed a machine-absolute Maven classpath and checked an unused absolute desktop profile. It now constructs the classpath from the pinned module/dependency manifest under the selected cache and excludes only that unused profile check. Engine/resource/dependency/configuration checks remain strict. Output runtime directories are recreated to prevent stale packaged files.

Actual relocation test: copied/hardlinked only required pinned inputs into a different cache root, omitted the old classpath and desktop profile entirely, packaged successfully, compared all engine/JAR/resource/preferences/opening-flag hashes, and built the deployable engine image from that portable output. [Portability record](V2-local-qa/portability.json). The final image runs without host class overlays or runtime-cache mounts. Final `/healthz` and `/readyz` report the correct pin/identity. Earlier Go-to-engine browser games and natural/lifecycle evidence retain their original hashes; no full suite was repeated.

A completely fresh source download/recompile was **not rerun** just to duplicate V2-010. The acquisition recipe reuses that proven command/toolchain, but cross-platform bit-identical recompilation is not asserted. If strict bytecode identity differs, packaging stops; do not bypass it. Build-time public network access and adequate local disk are required. No developer home path is required by the new preparation/packaging route. Historical logs may contain their original absolute paths.

Local raw logs/bundles: `/Users/zeusborrego/Developer/forge/v2-local-qa` (`cpu-js.log`, `portable-package.log`, `portable-image.log`, `web-build.log`, `portable-bundle`). The README explains portable paths; this evidence location is not a fresh-install dependency.

## Handoff

V2-043 is reviewable with the three missing browser paths and diagnostic delivery now verified. V2-042's local available-tool checks are complete, but its unavailable-device acceptance remains explicit. V2-050 public enablement remains blocked by owner G2/device/hosted checks, not another agent infrastructure phase. Both P1 fixes remain in review; Kaalia remains not reproduced and no GitHub issue was changed. All historical startup/headroom failures remain failures.
