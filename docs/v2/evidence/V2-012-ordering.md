# V2-012 — Simultaneous-trigger ordering and active-human measurements

**Ordering correction and the fixed human workload passed. Resource feasibility remains conditional, not accepted.** Five unchanged realistic browser runs reached the predefined turn-12 endpoint. Two of five 1 GiB runs failed the required 20% whole-container memory headroom. The bounded 2 GiB comparison is reported below separately; it cannot supply a repeat distribution or prove a hosting upgrade is necessary. Previous tasks remain in review, neither accepted nor merged.

Branch `codex/v2-012-resource-lifecycle`, HEAD `0ab74245912407e98f2cb5131a431a67cfed0f1f`. Raw artifacts: `/Users/zeusborrego/Developer/forge/v2-012-ordering`. Original `v2-012` and `v2-012-resume` artifacts and reports are preserved. This report supersedes their current prompt blockers, not their historical measurements.

## Scope and ordering proof

The correction maps only the pinned `PlayerControllerHuman.orderSimultaneousSa` → nine-argument `IGuiGame.order` call. A caller/mode guard rejects other ordering paths, mixed source/destination lists, non-ability or nonvisible entries. It handles Forge's offered source list and its cached proposed destination list, but always asks again. Every position starts blank. The player explicitly submits a complete permutation; the adapter returns the original offered `SpellAbilityView` references in that order and `rememberDecision=false`. There is no default ordering, rule patch or automated answer.

The pinned controller builds views and maps them back to the actual abilities at `forge-gui/src/main/java/forge/player/PlayerControllerHuman.java:2240–2337`. Its `orderAndPlaySimultaneousSa` at lines 2370–2402 iterates the returned list **from last to first**. `MagicStack` resolves its top. Consequently the browser says position 1 resolves next among these abilities, then position 2, subject to later responses. It does not equate returned order with stack insertion order.

Focused actual-engine/browser evidence:

1. A one-time fixture places existing precon Arahbo, Leonin, Sword and six lands using Forge APIs. Human keeps, advances to combat, resolves Eminence and declares the equipped Leonin attacking.
2. Player explicitly chooses **Sword first, Arahbo second**. Forge receives the original views, asks to place Arahbo then Sword, and shows Sword above Arahbo on the stack.
3. Sword resolves: human confirms the optional search and selects Forest from the authorized library view. Arahbo then resolves: human manually pays 1WG and Forge makes Leonin **12/12**. Sanitized `resolved-public-source` events corroborate this sequence.
4. Actual browser stale/invalid inputs return 409; competing submission returns one 200 and one 409. Protocol regression rejects ten malformed permutations, stale and replayed replies, and any remember field. It checks original object identity, a second explicit order despite Forge caching, and rejection of unrelated ordering callers.
5. The regression blocks the real controller on the AWT event thread and unblocks it from separate reply threads. The existing gate consumes exactly one reply, completes the waiting future directly, and does not queue the reply behind the blocked action thread. No reliance on `GameAction.invoke` as a per-game serial executor was introduced.

One implementation pass, one targeted fixture correction: the first fixture requested seven Plains from a deck containing three; the correction requests three. No second adapter redesign, Forge rebuild or gameplay modification. Current source hashes:

- Adapter: `b846f3167e49dc2c0939f5a15e90c9a6889e78999e2f53020926bff1c938ce36`.
- UI: `36bf7820d0729ad82b6641212daab33c4abfd8a2f3991f8da7401daaadb78c1a`.

[Protocol results](V2-012-ordering/protocol.json), [focused browser proof](V2-012-ordering/focused-checks.json), [projection regression](V2-012-ordering/projection.json), [artifact audit](V2-012-ordering/checks.json). The privacy regression retains hidden identities and legitimately scoped reveals. The new ordering path requires visible sources and uses the existing authorized label/projection functions; it does not weaken filtering globally. Distinct abilities with indistinguishable safe labels still fail explicitly. Other ordering families remain unsupported.

## Workload and measurement boundaries

The [workload definition](V2-012-ordering/workload.json) was recorded before the pilot. Fixed endpoint: first human turn-12 priority prompt, after six CPU turns. Unchanged V2-010 Feline Ferocity versus Open Hostility precons, 99+1 Commander, 40 life, Default AI, seed 20260912. No manufactured draws or removed cards in realistic games. Seed reproduction was observed for this path, not promised generally.

A successful unchanged pilot preceded the five fresh repetitions. Every run used actual browser controls operated by the agent: Keep, lands, Sol Ring, Sword, Leonin, manual mana, Equip target/payment, Eminence, Terramorphic sacrifice/search, two attacks, Sword search, Arahbo cast, explicit simultaneous-trigger ordering, optional payment decline and CPU actions. At the turn-10 Arahbo payment only two untapped mana sources remain; the human explicitly cancels the optional payment. This is a permitted decision, not an adapter fallback. Human life is 36 at the endpoint. No new unsupported prompt occurred. The informational “CPU picked” notice retains its previously recorded missing-name limitation; it does not require inventing a player selection.

These are automated **human-controller browser** workloads, not AI replacing the human and not an owner manual playthrough. They finish at a benchmark boundary and are cancelled, not natural match completions. Prior V2-011 focused natural human results and V2-012 full AI results keep their separate scope. A longer realistic natural match, all custom cards and every prompt family remain unverified.

Environment reused without Forge rebuild: pin `26d8aff87509dde8a9d5017852339382d7ab3e83`; V2-010 cache/resources/configuration; Linux AMD64 Temurin 21.0.6+7 on Rosetta inside ARM64 Docker Desktop 27.5.1 / LinuxKit 6.12.5. VM: eight CPUs and 4,109,344,768 bytes, **not** the enforced worker allocation. Image `eclipse-temurin@sha256:24a8854594eea72c16822953e6cb96c78d10fc3c77b7b8a60ce8e5ac440a2337`. One CPU, no swap, 256 PID limit, cgroup v2; heap 512 MiB at 1 GiB and 1 GiB at 2 GiB, `ActiveProcessorCount=1`, existing NMT flags. Supervisor 32 MiB heap inside the same cgroup. Exact arguments/configuration/artifact hashes are retained in each `command.json`; resource/source notices remain as documented in V2-010, with no binary distribution here.

Memory is kernel `memory.peak`, including native/JIT, file and kernel charges, supervisor and cgroup-charged emulation costs. It excludes the browser, site, DB and global Docker VM/translation caches. Nominal samples 250 ms. Active-window summaries aim to cover CPU turn 3 to endpoint cancellation, with missed sampled boundaries flagged below; last-30-second samples are descriptive, not proof of long-game memory plateau. One game per process; each worker and container is removed afterward.

Startup is host launch to first human GUI choice, detected at one-second intervals. Image was already pulled; filesystem caches were warm or unspecified. Docker was restarted before September 17 repetitions after an interruption, but this is **not** a controlled cold-storage experiment. Native AMD64/provider timing is unverified. CPU timing brackets `chooseSpellAbilityToPlay` with existing instrumentation; admission ACK timing is separate from engine completion and human thinking. First repetitions include browser-driver adjustment/documentation pauses in wall time, memory and data volume; those pauses are excluded from CPU/ACK latency. No samples or memory peaks were discarded for this reason.

## Results

All figures are engine plus supervisor only. Percentiles are nearest-rank. Five repetitions use 1 GiB; the 2 GiB row is one comparison, not a validated candidate distribution.

| Run | Ready seconds | Whole-container peak MiB | Headroom | Post-ready median MiB | Memory samples | Cleanup seconds | Result |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| human-1024-1 | 26.76 | 966.78 | 5.59% | 900.09 | 2029 | 1.19 | **Headroom fail** |
| human-1024-2 | 26.57 | 829.62 | 18.98% | 764.74 | 1707 | 1.28 | **Headroom fail** |
| human-1024-3 | 26.57 | 784.18 | 23.42% | 735.73 | 526 | 1.16 | Pass this run |
| human-1024-4 | 28.83 | 782.43 | 23.59% | 715.97 | 725 | 1.18 | Pass this run |
| human-1024-5 | 26.65 | 773.39 | 24.47% | 725.76 | 574 | 1.23 | Pass this run |
| human-2048-comparison | 27.73 | 778.34 | 62.00% | 696.54 | 718 | 1.18 | Headroom pass, n=1 only |

Five 1 GiB startups: p50 **26.65 s**, p95/max **28.83 s**, range **26.57–28.83 s**. Current five-run startup target passes for this cache/emulation scope; the earlier 32.84 s result remains retained under its older adapter/run identity. True cold/native startup remains unverified.

At 1 GiB, **645** CPU-decision samples give p50/p95/max **23.31 / 169.49 / 602.33 ms**. **555** successful command-admission ACKs give **8.36 / 29.78 / 72.37 ms**. These are method and proxy measurements, not end-to-end internet or complete CPU-turn timing. Each run supplies 129 CPU calls and 111 ACKs; the selected workload includes actual CPU development, attacks and decisions beyond the opening turns. The separate original five complete AI games remain the heavier supplemental engine corpus.

The 2 GiB comparison gives 129 CPU calls **26.02 / 235.80 / 488.48 ms**, and 111 ACKs **9.05 / 32.67 / 95.72 ms**. Its peak **778.34 MiB**, **62.00% headroom**, startup **27.73 s** and cleanup **1.18 s** pass the numeric targets in this one sample. This is the smallest tested *cap* whose completed-run evidence has no headroom failure, but it is **not yet a repeat-validated viable deployment configuration**. The lower peak does not prove the larger cap reduces usage: run duration, caches, polling, GC/JIT and emulation differ. No cause is asserted from this comparison.

All six runs reached the same intended endpoint, had no OOM, had observed worker disappearance, and removed their containers. Last-30-second live-memory summaries are in [active windows](V2-012-ordering/active-windows.json). The one-second view sampler missed the exact turn-3 boundary in four runs, so their derived active windows begin at the first observed turn 4 and are explicitly flagged approximate. Whole-process kernel peaks and CPU/ACK samples are complete and unaffected. The short last window does not establish a long-game plateau; no multi-game JVM is reused.

| Run | Wall seconds incl. tester pauses | Request payload bytes | Response payload bytes | Engine log bytes | All retained artifact bytes |
| --- | ---: | ---: | ---: | ---: | ---: |
| human-1024-1 | 525.20 | 25538 | 7579694 | 535 | 1417449 |
| human-1024-2 | 443.38 | 25538 | 8233232 | 534 | 1335648 |
| human-1024-3 | 140.45 | 25538 | 1530982 | 534 | 1004479 |
| human-1024-4 | 193.15 | 25538 | 1677784 | 535 | 1027463 |
| human-1024-5 | 153.26 | 25538 | 1494979 | 534 | 1005731 |
| human-2048-comparison | 191.25 | 25538 | 1273510 | 535 | 1016262 |

Payload excludes headers/TCP framing, health probes and failed proxy exchanges; artifact total is the supervisor's cleanup snapshot before later audit files. Full per-file sizes, RSS/thread maxima and sampled memory statistics are in [measurements](V2-012-ordering/measurements.json); aggregate distributions in [aggregate](V2-012-ordering/aggregate.json). Polling volume depends on human/tester thinking time; these figures are not a universal bytes-per-game or hourly budget. Raw logs/private diagnostics are kept outside Git.

## Reuse, checks and acceptance

The earlier five idle runs, five AI repetitions, 20 sequential cycles, cancellation during CPU work, SIGSTOP escalation, forced death, interrupted requests, refreshed browser and stale replacement controls retain their **historical adapter identities**. The unchanged supervisor/CPU timer/configuration means that evidence still informs containment; it is not relabeled as a full new-adapter rerun. This pass adds a failed setup cycle, a corrected focused cycle, a pilot and six completed-workload abort cycles, all reclaimed. Current ordering stale/competing replies and current-worker cancellation were verified. Final checks found no labeled experiment container or port-18711 listener. Retained files are intentional evidence. Production admission, authentication, retention and reconnect ownership remain unimplemented.

Native-Mac site evidence remains **exploratory and rate-limited**: 99/451 responses were HTTP 429 both alone and with the engine. It does not pass healthy existing-site load acceptance, cover authenticated saves or prove capped Linux cohosting. Engine-only results cannot be extrapolated to a combined web/DB service, multiple concurrent games or separate-service network latency. No site-load rerun or full lifecycle rerun was performed automatically.

| Criterion | Result / boundary |
| --- | --- |
| Explicit actual simultaneous abilities, original views, complete permutation, no remember | **Passed**, focused actual-controller regression and real browser fixture |
| Missing/duplicate/extra/stale/repeated/competing replies; safe blocked-thread handoff | **Passed**, malformed permutation corpus, one accepted racing reply and engine resume |
| Returned order, reverse placement, actual resolution | **Passed**, Sword search before paid Arahbo pump; repeated realistic sequence corroborates |
| Hidden identities filtered; legitimate reveals preserved; other ordering rejected | **Passed** in focused regression and recorded DTO audit; broad hidden-card/custom-card surface unverified |
| Unchanged pilot, then five realistic active-human repetitions to turn 12 | **Passed**, all actual browser controls; no new prompt blocker |
| Linux AMD64 capped measurements, explicit identity and emulation | **Passed locally**; native-host performance/cold storage unverified |
| 1 GiB target, at least 20% whole-container headroom | **Failed**, 2/5 runs; worst 5.59% headroom |
| Bounded 2 GiB comparison | **Passed this sample**; repetition requirement at a viable candidate **unverified**, n=1 |
| Startup ≤30 s and ACK p95 <1 s | **Passed current measured corpus**, cold/native startup unverified; prior slower start retained |
| Cleanup ≤30 s, no OOM/leftover worker or occupied capacity | **Passed current cycles**, historical fault evidence reused |
| Long-game memory plateau / broad Commander capacity | **Unverified**, fixed six-CPU-turn corpus is bounded |
| Existing healthy traffic / actual hosting topology | **Unverified**, prior Mac comparison rate-limited; no native capped cohosting |
| Required owner manual playthrough / G1 | **Unverified / not approved**, no hosting or release acceptance implied |

## Reproduction and handoff

Run [the benchmark instructions](../../../tools/forge-benchmark/README.md#completed-ordering-workload-and-repeat-recipe) with the retained cache and artifact root. Exact measurement command:

```sh
python3 tools/forge-benchmark/run.py run \
  --artifacts /Users/zeusborrego/Developer/forge/v2-012-ordering \
  --name NEW-UNUSED-NAME --memory 1024 --scenario realistic --timing --seconds 1800
```

For the one comparison substitute `--memory 2048`. Use actual browser controls from the [manual turn-12 sequence](../../../tools/forge-browser-spike/README.md#v2-012-ability-correction-and-ordering-reproduction). After the endpoint, `finish_workload.py /absolute/run/path` captures it and reclaims the process. The engine-backed focused protocol command is `regression.py --test ordering --out /absolute/new/path`; privacy/presentation uses `--test projection`. These compile only local adapter/test classes. `verify_ordering.py ARTIFACTS --out docs/v2/evidence/V2-012-ordering` reproduces the numeric and DTO audit without gameplay. Source pins, profile/resource/deck checks, image identity and component notices remain in V2-010 and the original benchmark report.

**Task state: blocked for acceptance; authorized measurements and ordering change are complete and ready for review.** No further prompt correction is indicated by this corpus. The precise unmet condition is a repeat-validated resource candidate meeting headroom plus an acceptable existing-site/hosting boundary; 1 GiB fails, 2 GiB has one promising comparison, and healthy/native topology remains unverified. Do not spend more time implementing prompts to address a resource-evidence gap.

Smallest justified next action: owner scope decision on a native Linux AMD64 validation pass using already available capacity, if any, with five representative 2 GiB runs and a healthy equal-load site comparison. First verify the actual DigitalOcean allocation and available headroom read-only; do not purchase or upgrade just to fill the evidence gap. This is a recommendation for subsequent authorization, not execution of V2-013. V2-013 remains ineligible under the passing-review dependency rule until the remaining condition is resolved or explicitly dispositioned by the owner. No G1 approval or M2 work.

The owner permits considering a DigitalOcean upgrade if necessary and wants eventual signed-in-user access as usage grows. These results do **not** establish that an upgrade to the current web service is necessary or select a plan. The reported 512 MiB web allocation cannot be treated as spare Forge capacity; the original 512 MiB engine screen already failed headroom. Signed-in eligibility does not establish simultaneous capacity: the roadmap still proposes one active match per account plus a measured global cap and an honest busy response. Multiple games, demand-driven scaling and additional monthly cost require separate evidence and owner choice.

One primary agent, no delegates. Work reused the pin, dependencies, resources and compiled Forge modules. One implementation and one fixture correction; later browser-locator adjustments affected only test operation, not the adapter. September 15 focused/pilot work and September 17 repetitions were separated by user interruption; paused time is not active investigation or engine latency. Exact token/cost counters unavailable. The audit verifies all preexisting files against the recorded baseline, allowing only the scoped source/UI/helper/status/documentation edits. No app bug fixes, merge, publication, deployment or paid provisioning. Surtr and failed-note-save P1 defects remain blockers; Kaalia remains not reproduced.
