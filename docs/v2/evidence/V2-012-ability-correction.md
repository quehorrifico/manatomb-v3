# V2-012 — Ability projection correction and resumed human measurements

**September 15, 2026 — blocked, with a verified narrow correction.** The original unidentified ability is now usable through real browser controls. The unchanged realistic Commander decks progressed to turn 10 with casting, mana payment, equipment, searches, a commander, attack triggers and CPU combat. Forge then requested **simultaneous-trigger ordering (`IGuiGame.order`)**, which this adapter does not implement. The harness failed visibly and reclaimed the worker. Five completed representative human repetitions and the planned turn-12 endpoint remain unverified. No V2-013 eligibility or G1 approval.

The [initial V2-012 report](V2-012.md) and all 20 historical cycles remain preserved. New external artifacts are in `/Users/zeusborrego/Developer/forge/v2-012-resume`; compact [measurements](V2-012-resume/measurements.json), [checks](V2-012-resume/checks.json), [workload definition](V2-012-resume/workload.json), [active window](V2-012-resume/active-window.json), [regression](V2-012-resume/regression.json) and [browser boundary observations](V2-012-resume/browser-checks.json) accompany this report. These are additional partial measurements, not replacement or completed benchmark repetitions.

## Baseline, scope and exact cause

Branch remains `codex/v2-012-resource-lifecycle`; ManaTomb HEAD remains `0ab74245912407e98f2cb5131a431a67cfed0f1f`. Forge pin remains `26d8aff87509dde8a9d5017852339382d7ab3e83`. The original adapter was copied to external `BrowserSpike.before.java`; all preexisting roadmap/tool/evidence files were hashed before editing. Only the authorized adapter presentation, associated instructions, status and report pointers changed. No application bug fixes or Forge source/resource changes.

The first new run, `original-reproduction`, used the original adapter. Actual browser controls reproduced the exact recorded path: Keep the deterministic hand, yield through CPU turn 1, play Plains on human turn 2, End Turn. At `COMBAT_BEGIN`, `getAbilityToPlay` offered option **0**, `min=0`, `max=1`, text **` [Phase: ]`**. It was left unanswered and the worker cancelled.

The pinned source and the real-Forge regression establish the chain:

1. `PlayerControllerHuman.getAbilityToPlay`, lines 214–226, maps actual abilities to `SpellAbilityView`, calls the GUI, and maps its returned view back to the original ability.
2. `SpellAbilityView.updateDescription`, lines 53–57, stores `sa.toUnsuppressedString()`. Its host card is separately available via `getHostCard()`.
3. Arahbo's script, `forge-gui/res/cardsfolder/a/arahbo_roar_of_the_world.txt`, has a primary battlefield BeginCombat trigger with `TriggerDescription`. Its **secondary command-zone BeginCombat trigger lacks `TriggerDescription`** and executes the same `TrigPump1` ability.
4. `Trigger.toString(boolean)`, lines 121–123, returns empty text without that parameter. `WrappedAbility.getStackDescription(false)`, lines 186–218, appends the important trigger annotation. `TriggerPhase.getImportantStackObjects`, lines 70–74, appends the triggering player's string after `Phase:`; this human's personalized engine name is empty. The actual resulting view description is ` [Phase: ]`.
5. The old adapter omitted the separate source card. Its metadata regex and hidden-name scrub **did not remove the missing description**. The new live `ability-context` records and the regression show raw view text ` [Phase: ]`, `metadataRemoved=false`, `scrubChangedDescription=false`. This is an upstream missing description plus lost source context, not a sanitizer failure.

## Focused correction and verification

[BrowserSpike.java](../../../tools/forge-browser-spike/src/BrowserSpike.java) now prefixes visible ability choices with the authorized current-face source name. When the description contains no prose beyond bracket annotations, it adds that visible face's **Forge Oracle text**, explicitly labeled “Source rules.” It does not synthesize a trigger description, match card names, parse card rules or alter gameplay. At the reproduced sole BeginCombat option, the source, phase and full card text distinguish the Eminence choice from the card's attack ability. The presentation makes no promise that arbitrary missing descriptions are now unambiguous; identical projected ability labels fail explicitly.

Invisible sources still produce only “Hidden ability.” Existing visibility checks, authorized reveal scopes, metadata removal and hidden-name sanitization remain in place. Only strings/whitelisted DTOs are projected. Option IDs, 0–1 optionality, original view references, the return-value future and action/reply serialization are unchanged. No automatically selected option, empty fallback reply, input default, new engine rule or browser UI was added.

New adapter SHA-256: `f857c0ad0b7173e050abcdbcd8e1140eb8e7cdd76f6a053d43f9158fe32dac58`.

- **Real browser regression passed:** corrected turn-2 radio displayed Arahbo and source rules. Invalid choice and stale revision returned 409 without consuming it. Selecting that radio and pressing the competing-reply test produced one 200 and one 409, then Forge advanced to CPU turn 3. These are agent-operated real browser controls, not owner manual acceptance or a direct HTTP gameplay driver.
- **Real-Forge focused regression passed:** [test source](../../../tools/forge-browser-spike/src/AbilityProjectionRegression.java) loads the pinned card script, constructs its actual secondary trigger and `WrappedAbility`, and verifies exact upstream/view text and authorized context. It checks original returned object identity, optional empty reply, invalid/stale/replayed inputs, two competing replies with exactly one acceptance, indistinguishable-choice refusal, hidden card/ability/prose exclusion and explicit reveal authorization followed by revocation. It is an integration/unit regression running on native Mac, not a capacity benchmark or a full game.
- **New human DTO audit passed:** 100 projected views; CPU library always absent, the single CPU hand reveal limited to Forge's land-search confirmation, and the human library visible only during its two actual search prompts and sorted by name rather than physical order. Card and stack field whitelists remained intact. These checks cover the exercised paths, not every Forge privacy mechanism.
- All 66 recorded `browser-action` inputs in the extended pilot were accepted by Forge; 99 successful commands include synchronous choices. The original separate safe reply path unblocked both actual human-input machinery and synchronous choices. No Forge rebuild occurred.

## Defined workload and results

The workload was written before the corrected pilot's active segment. It uses unchanged Feline Ferocity versus Open Hostility (99+1), Commander, Default CPU, seed `20260912`, a normal 40-life game and natural draws. Plan: develop legal lands, creatures and equipment, pay costs, target, attack/block as appropriate, and reach global turn 12 including six CPU turns. First establish coverage, then five fresh repetitions. Stop on the first unsupported/ambiguous decision rather than simplify the decks or repeat a known failure.

`corrected-browser` passed the correction/boundary checks, then was interrupted at the turn-4 Sol Ring spell choice. Its 2,400-second deadline reclaimed it. Its long waiting interval is not engine latency; a native regression also overlapped that diagnostic run, so it is not a clean repeat. `human-pilot-2` resumed from a fresh process, used no overlapping native regression, and reached the concrete ordering blocker after about eight minutes. Both partial runs are preserved; neither is counted as a completed representative repetition.

All rows below are **1 GiB / one CPU, Linux AMD64 emulated by Rosetta in ARM64 Docker Desktop**, engine plus supervisor in the same capped cgroup, no swap. Image/JDK remains `eclipse-temurin@sha256:24a8854594eea72c16822953e6cb96c78d10fc3c77b7b8a60ce8e5ac440a2337`, Temurin `21.0.6+7-LTS`; Docker client/server `27.5.1`, Python `3.14.0`. Engine `-Xmx512m`, `ActiveProcessorCount=1`, NMT summary, V2-010 module flags/preferences/resources; supervisor `-Xmx32m`, Serial GC. The unchanged CPU timing agent brackets `chooseSpellAbilityToPlay`, including entry-marker overhead. Runtime command/config/build identities are in each external `command.json`, `container-initial.json` and `config/engine.argv`; hashes are linked in the checks. Only adapter/instrumentation bytecode was compiled with the retained JDK, targeting Java 17. No dependency or Forge module was rebuilt.

| Metric | Interrupted correction pilot | Extended human pilot (`human-pilot-2`) |
| --- | --- | --- |
| Completed representative repetitions | 0; interrupted at turn 4 | 0; ordering failure at turn 10, target was turn 12 |
| Host fresh-process startup to first GUI prompt | 29.12 s, n=1, diagnostic overlap | **32.84 s**, n=1; exceeds proposed 30 s target |
| Whole-container kernel peak | 792.57 MiB | **809.24 MiB**, 20.97% headroom; only 9.96 MiB below the 80%-of-cap line |
| Samples / worker maximum RSS / threads | 9,308 / 417.82 MiB / 25 | 1,927 / 430.07 MiB / 26 |
| Active window: CPU turn 3 to failure | Interrupted; do not interpret long idle as active latency | 1,632 samples; memory min/median/max **709.21 / 772.81 / 808.78 MiB**; 62.50 cgroup CPU seconds over 419.27 s wall time |
| Final 60 seconds of live window | Not a workload plateau proof | Min/median/max **801.54 / 803.61 / 808.78 MiB**; one evolving game, not proven eventual plateau |
| Actual CPU decision p50 / p95 / max | 43.88 / 302.11 / 391.80 ms, n=31 | **29.62 / 157.45 / 303.37 ms, n=109**, includes CPU turns before active-window boundary |
| Successful command ACK p50 / p95 / max | 19.12 / 105.47 / 105.47 ms, n=16 | **14.06 / 45.35 / 134.80 ms, n=99** |
| Worker cancellation-to-reap / container cleanup | 0.565 / 1.883 s | **0.284 / 1.662 s**, terminal failure reclaimed automatically |
| Recorded request / response payload bytes | 4,344 / 3,374,754 | **22,794 / 11,020,432** |
| Raw output/log bytes | See per-file JSON; includes long sampling wait | **1,108,326** across samples, decisions, events, projected transcript and engine log |
| Natural human result / OOM / leftover container | None / none / none | None / none / none |

Peak includes container file/kernel/native memory and supervisor, not merely Java heap. The browser, host tools, Docker VM/global caches and existing ManaTomb are outside that cgroup. These are fresh processes with an already pulled image and warm/unspecified storage caches. CPU timings are method durations, ACKs are admission durations; neither includes human thinking. The active-window wall duration includes agent/browser tool delays and is not engine resolution time. One-second phase observation bounds and nominal 250 ms memory samples are explicit. Payload counts exclude transport framing, internal readiness probes and failed proxy exchanges; they depend on polling and human time and are not a bytes-per-finished-match estimate.

The old 512 MiB headroom failure remains valid. **1 GiB remains conditional, with much thinner measured headroom; no smallest viable deployment is established.** No 2 GiB experiment is justified by the semantic blocker alone. One startup exceeded the target, so the earlier five sub-30-second idle starts cannot be generalized to a new all-runs startup pass. Native AMD64/provider timing and cold storage remain unverified.

## New blocker and smallest next experiment

At turn 10, Arahbo was on the battlefield and the equipped Leonin Shikari attacked. The preceding public state and pinned scripts establish the simultaneous Arahbo and Sword attack triggers. Actual stack trace:

```text
UnsupportedOperationException: Unmapped GUI method: order
BrowserSpike.invoke
PlayerControllerHuman.orderSimultaneousSa:2313
PlayerControllerHuman.orderAndPlaySimultaneousSa:2371
MagicStack.chooseOrderOfSimultaneousStackEntry:881
```

`IGuiGame.order(...)` returns `OrderResult<SpellAbilityView>` containing the ordered view list and remember-decision flag. The human controller maps that list back to the original abilities; `orderAndPlaySimultaneousSa` then plays them in reverse list iteration. The current spike deliberately has no general ordering UI. It did not return null, use Forge's possible default order, reorder automatically, skip the attack, remove an equipment or change the deck. The first such failure ended this bounded pass. No five repetitions of a known failing corpus were run.

**Smallest next action:** a separately scoped adapter follow-up for this actual simultaneous-ability ordering call: display only permitted source/ability context, accept an explicit complete permutation with stale/duplicate validation, return the original views with verified resolution order, and retain the fail-closed behavior for other ordering modes. Then resume this fixed realistic corpus and its five required repetitions at 1 GiB. Do not silently enable saved ordering or broaden to arbitrary card ordering. No engine change appears necessary from the inspected interface, but that implementation is unverified and is not included here.

One additional presentation limitation was observed: the supported informational `message` prompt said “CPU picked” without a player name when Saskia resolved. `ChoosePlayerEffect` builds that notice from the player's engine string; the personalized human name is empty, as in the phase annotation. The agent explicitly acknowledged this notification; it supplied no choice of player and made no claim about the unshown selection. Preserve this separate visibility/presentation question for follow-up; it was not treated as proof of complete player-choice presentation.

## Evidence reuse, acceptance and handoff

| Criterion | Result after this pass |
| --- | --- |
| Reproduce, trace and correct `[Phase: ]` without gameplay changes | **Passed** in original/corrected real browser runs and real-Forge regression |
| Preserve option identity/optionality and safe reply boundaries | **Passed** for focused regression and exercised browser race/stale/invalid controls |
| Relevant hidden information and authorized reveals | **Passed** for regression and audited new public projections; exhaustive coverage unverified |
| Capped environment, retained build/resources/configuration identity | **Passed locally** with explicit emulation; native/provider timing unverified |
| Representative active human workload and five completed repetitions | **Failed at `order` / unverified repetitions**; turn-12 endpoint not reached |
| Whole-container headroom | **Passed for observed partial peak only**, narrow 20.97% margin; later workload unverified |
| Proposed startup target | **Failed in one new sample** (32.84 s); repeat distribution unverified |
| ACK/CPU/memory/cleanup/volume measurements | **Passed for recorded partial scopes**, not full-game estimates |
| Lifecycle/repeated cycles | **Prior evidence retained**: 20 original cycles, plus 3 new reclaimed processes. No full fault-suite revalidation of new adapter claimed |
| Healthy existing-site/cohosting gate | **Unverified**, unchanged native-Mac exploratory result was rate-limited |

Unchanged Forge engine/CPU/resource settings preserve the relevance of five old idle and five old AI runs, the original 20 lifecycle cycles and their fault outcomes. They retain their original adapter hash and scope. AI execution does not load `BrowserSpike`, and supervisor/fault mechanics were unchanged. New ability prose/logging and browser traffic can affect human memory/volume; only the new rows measure those costs. The old V2-011 full-corpus audit is tied to the historical adapter hash and should not be presented as a rerun against this correction. The new regression covers its changed seam. Prior tasks remain **review**, not accepted or merged.

Production ownership, admission, deadlines, reconnect authorization, watchdogs and retention remain separate work. No capped Linux cohosting, authenticated deck/save traffic, healthy-site load gate, full wire accounting or hosting capacity is inferred from this engine-only experiment. V2-010 component notices/source/resource handling continue to apply; no image or runtime is published.

One primary agent; no delegates. Initial correction pass and resumed September 15 coverage were bounded; user interruption is recorded separately from execution. No exact token/cost counter is available. Both P1 release blockers (Surtr normalization and failed saved-note loss) remain unresolved; Kaalia remains not reproduced. No merge, deployment, paid provisioning, GitHub update, V2-013 work or release approval occurred.
