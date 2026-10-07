# V2-012 local resource/lifecycle experiment

This runner reuses the V2-011 adapter and preserves Forge source. The owner-authorized V2-012 follow-ups add ability presentation and the specifically authorized simultaneous-trigger ordering boundary; see the [correction report](../../docs/v2/evidence/V2-012-ability-correction.md). It runs one engine plus a small local measurement/HTTP supervisor in the **same** Linux AMD64 cgroup. It never answers a gameplay prompt. See [the evidence report](../../docs/v2/evidence/V2-012.md) for the conditional results and current human-workload blocker.

## Requirements and preparation

- Docker with `linux/amd64` execution, cgroup v2 memory/CPU enforcement and a local socket/context. Docker Desktop on ARM uses emulation; timings are not native deployment evidence.
- The retained V2-010 cache and its exact configuration. The runner checks those configuration hashes and obtains the original command through the unchanged V2-010 helper.
- JDK 21.0.6 tools for compiling the two local instrumentation classes and the current adapter. Forge modules/resources/dependencies are **not** rebuilt. No Maven/network acquisition occurs during a measured run.
- Python 3 and Docker CLI on the host. Image is pinned by digest; no authenticated registry connection or paid resource is required.

```sh
cd /Users/zeusborrego/Developer/manatomb-v3
docker pull --platform linux/amd64 eclipse-temurin:21.0.6_7-jdk-jammy
python3 tools/forge-benchmark/run.py prepare \
  --javac /Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home/bin/javac
```

Default cache: `/Users/zeusborrego/Developer/forge/v2-010`; artifacts: `/Users/zeusborrego/Developer/forge/v2-012`. Use `--cache`, `--artifacts`, `--javac` on another host. The cache must be transferred intact, including source/resources, bytecode and runtime Maven jars. Platform-independent bytecode is reused; selected launch paths are translated explicitly to `/forge`, `/repo`, `/support`, `/runprofile` and `/out`. A private per-run profile relocates paths only; preference/deck contents remain unchanged. The old Mac-specific helper/source files are untouched.

The Linux JVM uses the same Temurin version as V2-010. `--memory` accepts only 512, 1024 or 2048 MiB; one CPU, no swap, 256 PID limit. Engine heap is one-half the cap; `ActiveProcessorCount=1` and Native Memory Tracking are recorded. The supervisor uses a 32 MiB heap/Serial GC and is included in the cgroup measurement. Headroom decisions use **memory.peak**, including anon/file/kernel memory, not heap or Docker's cache-subtracted display.

The experiment's image pin is `eclipse-temurin@sha256:24a8854594eea72c16822953e6cb96c78d10fc3c77b7b8a60ce8e5ac440a2337`, always with `--platform linux/amd64`. Verify the resolved image/runtime on a new host; retained Mac-specific jars on the classpath are not proof every optional native library works on Linux. Tested headless paths loaded successfully.

## Runs

```sh
# Initial screen; 15 seconds at the actual first human GUI choice.
python3 tools/forge-benchmark/run.py run --name my-screen-512 \
  --memory 512 --idle-seconds 15 --seconds 180

# Only increase after the preceding candidate fails its screen/headroom requirement.
python3 tools/forge-benchmark/run.py run --name my-human-1024 \
  --memory 1024 --scenario realistic --seconds 1800

# Supplemental CPU load; never count as browser/human-controller evidence.
python3 tools/forge-benchmark/run.py run --name my-ai-1024 \
  --memory 1024 --mode ai --timing --seconds 180
```

For human runs open **http://127.0.0.1:18711/** and use the [V2-011 controls](../forge-browser-spike/README.md). The supervisor forwards to the adapter's container-loopback port; only the supervisor's mapped host port is exposed, bound to host loopback. Do not answer an unidentified/unsupported prompt. The `[Phase: ]` ability now includes authorized source rules. The authorized simultaneous-trigger ordering path now has explicit permutation controls. Follow the turn-12 sequence in the browser instructions; other unsupported ordering modes still stop explicitly. Performance acceptance is separate from completed workload coverage.

Run names must be new. Only one labeled experiment container is admitted. The host runner holds a file lock, refuses occupied capacity, ends on natural process exit/terminal view/deadline, and stops/removes the exact container in `finally`. Ctrl-C initiates cleanup. Recovery after a killed host supervisor:

```sh
python3 tools/forge-benchmark/run.py stop
```

This removes **only** containers labeled `manatomb.benchmark=V2-012`. Unrelated Docker containers and the retained Forge cache are preserved. Local artifact directories are intentionally retained, not leaked game sessions. No automatic retention policy or production ownership/admission service is implemented.

## Repetitions and lifecycle probes

Use five fresh names per chosen workload. For the measured repeated idle workload use `--scenario realistic --idle-seconds 15`; it waits at Forge's authorized setup reveal, before mulligan. No synthetic Keep response is injected. For repeated supplemental full games use `--mode ai --timing`; inspect both natural results and engine logs. A Java agent brackets **only** `PlayerControllerAi.chooseSpellAbilityToPlay`; returned values/card rules are unchanged. It adds entry-marker I/O to call timing and adds instrumentation overhead to total runtime. An uninstrumented simulation is retained as a comparison.

While a held human run is ready, in another terminal:

```sh
python3 tools/forge-benchmark/faults.py capture --run /path/to/run
python3 tools/forge-benchmark/faults.py interrupt --run /path/to/run
python3 tools/forge-benchmark/faults.py stuck --run /path/to/run
```

`capture` stores old controls in a private file (mode 0600). `interrupt` closes an incomplete HTTP request and checks the same prompt remains. `stuck` sends SIGSTOP to this worker, then requests cancellation: the supervisor waits three seconds and escalates to SIGKILL if necessary. Use `cancel` for normal cancellation or `kill` for immediate worker loss. In a fresh session:

```sh
python3 tools/forge-benchmark/faults.py replacement --run /path/to/new-run \
  --previous /path/to/old-run/retired-control.private.json
```

This tests both the old token and a fresh token with the old session/prompt. Neither is a new valid game action. For cancellation during a CPU call, launch an AI run with `--timing`, then run `faults.py cpu-cancel` or `cpu-kill` with its run path; it waits for a recorded unmatched method-entry marker. A bounded timeout reports missing coverage instead of pretending it interrupted CPU work.

Refresh/reconnect must also be exercised through actual browser refresh against a held prompt. An HTTP state probe alone is not an independent user/browser test. Fault endpoints are disposable, loopback-only supervision; they are not production session controls.

## Metrics and reporting

```sh
python3 tools/forge-benchmark/summarize.py /Users/zeusborrego/Developer/forge/v2-012
```

Each run retains exact container/JVM argv, adapter/instrumentation hashes, Docker state/limits, cgroup samples at nominal 250 ms, event/HTTP timing, engine log and cleanup evidence. A one-second ready detector records startup; measured repeats also record host-observed launch-to-ready time. The supervisor's own clock begins after JVM bootstrap, so older supervisor-only readiness figures omit Docker/JVM launch. All runs are fresh processes, with an already pulled image and warm/unknown host filesystem cache; they do not prove cold-provider boot or cold-storage timing.

Container memory includes the supervisor, native memory, emulation process overhead charged to the cgroup, page cache and kernel charges. JVM RSS and Native Memory Tracking are supplementary; they omit other container/VM costs. Recorded successful HTTP payload bytes exclude HTTP/TCP framing, internal health probes and failed proxy exchanges. Log/artifact bytes are measured separately. Command admission latency is distinct from a subsequent view or engine decision; it must not include human thinking time.

The optional `site_load.py` permits localhost only and sends read-only card autocomplete/health traffic at 10 requests/sec. Run equal-duration baseline and with-engine samples on the same catalog. It is **native-Mac, separate-process exploratory evidence**, not capped cohosting, authenticated deck/save coverage or production performance. Production-like Linux Go + Forge topology remains unmeasured.

Do not publish `*.private.*`, raw engine logs, old tokens, or arbitrary Forge traces. Checked-in evidence contains numeric summaries and sanitized observations only. V2-010's source/notices/resource requirements continue to apply; this task publishes no image or runtime bundle.

## Resume artifacts and focused verification

Keep the original `/Users/zeusborrego/Developer/forge/v2-012` results intact. New correction artifacts use `/Users/zeusborrego/Developer/forge/v2-012-resume`. Prepare again only when local adapter/instrumentation inputs change; this does not rebuild Forge.

```sh
python3 tools/forge-benchmark/run.py prepare \
  --artifacts /Users/zeusborrego/Developer/forge/v2-012-resume \
  --javac /Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home/bin/javac
python3 tools/forge-benchmark/run.py run \
  --artifacts /Users/zeusborrego/Developer/forge/v2-012-resume \
  --name my-human-order-reproduction --memory 1024 \
  --scenario realistic --timing --seconds 3600
python3 tools/forge-browser-spike/regression.py \
  --out /Users/zeusborrego/Developer/forge/v2-012-resume/my-regression
python3 tools/forge-benchmark/summarize.py \
  /Users/zeusborrego/Developer/forge/v2-012-resume
```

Run the native regression outside a measured gameplay interval. For the retained September 14–15 artifacts, reproduce the checked-in numeric/projection audit with:

```sh
python3 tools/forge-benchmark/verify_resume.py \
  /Users/zeusborrego/Developer/forge/v2-012-resume \
  --out docs/v2/evidence/V2-012-resume
```

This artifact audit expects the recorded run names and `regression-complete`, confirms the exact observed failure, and explicitly does **not** mark full V2-012 acceptance passed. New experiments should use fresh names and update the workload/evidence deliberately, not overwrite old measurements. Follow the [manual reproduction](../forge-browser-spike/README.md#v2-012-ability-correction-and-ordering-reproduction). Do not rerun five copies of the known unsupported ordering path without a relevant correction.

## Completed ordering workload and repeat recipe

Latest evidence: [V2-012 ordering and representative measurements](../../docs/v2/evidence/V2-012-ordering.md). Preserve both earlier artifact roots. Current compiled support and new outputs are at `/Users/zeusborrego/Developer/forge/v2-012-ordering`; no Forge rebuild is needed. If preparing on a new host, use the same prepare command with this artifact path.

```sh
python3 tools/forge-benchmark/run.py run \
  --artifacts /Users/zeusborrego/Developer/forge/v2-012-ordering \
  --name manual-human-1 --memory 1024 --scenario realistic --timing --seconds 1800
# In another terminal, only after actual browser play reaches human turn 12:
python3 tools/forge-benchmark/finish_workload.py \
  /Users/zeusborrego/Developer/forge/v2-012-ordering/manual-human-1
```

The finish helper verifies the observed endpoint, saves the human DTO and cancels/reclaims that local worker. It submits no gameplay. Repeat with five fresh names and the unchanged browser sequence. The ordering fixture is a separate `--scenario ordering` run. A 2 GiB comparison uses `--memory 2048`; the runner also changes heap from 512 MiB to 1 GiB, so this is a configured-candidate comparison, not a memory-cap-only experiment.

Reproduce the retained numeric/privacy/ordering audit (expects the recorded six measurement names) with:

```sh
python3 tools/forge-benchmark/verify_ordering.py \
  /Users/zeusborrego/Developer/forge/v2-012-ordering \
  --out docs/v2/evidence/V2-012-ordering
```

The original idle, AI and lifecycle evidence remains under its original hashes. New active results do not retroactively retest those workloads or validate native DigitalOcean timing, long-match memory plateau, multiple concurrent games or healthy cohosting. Do not use an emulated engine cap as a hosting upgrade recommendation.
