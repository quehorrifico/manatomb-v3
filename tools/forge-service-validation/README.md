# Local packaged-service validation

Only disposable **local Unix-socket Docker** is supported by these helpers. They do not connect to hosting accounts. Full fresh-topology setup is in `services/forge/README.md`. The named checkpoint topology uses `manatomb-v2-release-local`, web on loopback 18880, engine on loopback 18881, and an isolated test PostgreSQL database. Private environment files stay outside Git.

`restart-local.py --env-dir /absolute/private/directory --web` recreates only the named local web/engine containers, retaining the test database/network. It refuses to interrupt a live JVM. End the game through the browser first. `--fixture focused|blocking|ordering|choices` mounts a test-only launcher; omit it for normal Forge randomness. Production launch never selects a fixture.

`observe.py --container manatomb-forge-release-local --output /new/path.jsonl --seconds 1800` observes the complete cgroup every five seconds. Limits: 1–3600 seconds, fresh output, no gameplay actions. Diagnostics and process-list overhead remain charged. The cgroup peak is container lifetime, not automatically an individual session's peak. Native Mac and AMD64 emulation results must be labeled separately.

`capture-result.py --env-dir /absolute/private/directory --scenario 'description' --output /new/path.json` captures an already terminal owner-1 local test result, image/runtime identity, resource limits and worker/file cleanup without decks, hands, tokens or prompt text. It never submits gameplay decisions. `lifecycle.py --help` describes bounded process-fault checks; actual browser gameplay is separately recorded in `docs/v2/evidence/V2-M2.md`.

## Choices fixture manual playthrough

This deliberately arranged initial state tests adapter boundaries, not realistic Commander duration or capacity. Build the current package, restart with `--fixture choices`, then enter:

Human commander: `Silvos, Rogue Elemental`. Main: `93 Forest`, one each of `Doubling Season`, `Hardened Scales`, `Grizzly Bears`, `Walking Ballista`, `Vines of Vastwood`, `Evolution Charm`.

CPU commander: `Isamaru, Hound of Konda`. Main: `99 Plains`.

Forge validates both 100-card lists. The fixture arranges ten Forests, both replacement enchantments and Bears on the human battlefield; Ballista, Vines and Charm in hand; CPU starts at eight life. No later state changes originate in the fixture.

1. Choose play and keep. Reach main-phase priority through actual controls.
2. Cast Walking Ballista, choose X=2, and pay the requested four mana manually. At the competing replacement prompt explicitly choose the sequence. Scales then Season should produce six counters; confirm the **actual** board result. The order is application order, unlike simultaneous-trigger stack placement.
3. Exercise Evolution Charm's mode/target choice and Vines' optional kicker payment. Read all new prompts; never automatically choose an option on an unknown prompt.
4. Finish through legal attacks/abilities or explicitly cancel. Record natural results separately from concessions and cancellation. Download only the sanitized diagnostic summary.
5. End/reclaim and restart without `--fixture` before representative play.

This is a procedure, not a claim that every step has passed. Evidence records the exact exercised sequence and current hash. Other fixture setups are defined by the corresponding `setup…Fixture` methods and historical evidence; they do not modify Forge card scripts.

## Healthy local site comparison

`site-workload.py --mode baseline --output /new/baseline.json` requires no active JVM. Repeat with `--mode human-prompt` while the browser is holding a reached decision. It asserts the same decision remains throughout; it never answers prompts. Each pass makes ten card searches, ten authenticated deck reads and ten saves, at most one request per second, using only the named disposable database and synthetic owner/deck fixture. Saves preserve and verify the existing overview. Login is outside the timed samples. Any unexpected status or changed fixture fails the pass; HTTP 429 is not a successful performance sample.

Keep the web/engine images and resource settings unchanged between passes. This measures a separate-service local topology, warm/unspecified storage, a tiny catalog and an idle human boundary; it does not establish production database behavior, active CPU contention, native AMD64 capacity or end-to-end browser/network latency. Browser polling and cgroup instrumentation are additional work, not part of the HTTP body-byte counters. Use `observe.py` alongside gameplay for whole-container accounting.

`operator-smoke.py --env-dir /absolute/private/directory --output /new/operator.json` requires zero workers on the named local engine. It temporarily drains admission, checks liveness/readiness and rejection without allocating a worker, then restores admission in `finally`. It uses only loopback/local Docker and never calls hosting. Keep its artifact distinct from actual browser gameplay.
