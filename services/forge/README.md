# ManaTomb Forge service (local implementation in progress)

One authenticated supervisor, one JVM per active game, initial global cap one. This is application work under conditional G1 acceptance, not hosted/public release approval. See `docs/v2/STATUS.md` for current acceptance evidence. Never deploy an intermediate image merely because it builds.

## Pinned runtime and build

Use Python 3.12+ and **Eclipse Temurin JDK 21.0.6+7** (not just a JRE), plus Docker with Linux AMD64 build support. Obtain the appropriate JDK archive from the [official pinned Temurin release](https://github.com/adoptium/temurin21-binaries/releases/tag/jdk-21.0.6%2B7), check its published SHA-256, and extract it to a directory you choose. The runtime Docker base is separately pinned by digest in the Dockerfile. No global Maven installation is required.

On a **fresh installation**, from this repository root, choose dedicated paths and run:

```sh
# Replace these three paths with your own; no ManaTomb developer path is required.
FORGE_CACHE=/absolute/path/to/new-forge-cache
FORGE_BUNDLE=/absolute/path/to/new-forge-bundle
FORGE_JDK=/absolute/path/to/temurin-21.0.6-jdk
python3 services/forge/prepare-runtime.py --cache "$FORGE_CACHE" --java-home "$FORGE_JDK"
python3 services/forge/package.py --cache "$FORGE_CACHE" --output "$FORGE_BUNDLE" --javac "$FORGE_JDK/bin/javac"
docker build --platform linux/amd64 -t manatomb-web:v2-review .
docker build --platform linux/amd64 \
  --build-context forge-runtime="$FORGE_BUNDLE" \
  -f services/forge/Dockerfile -t manatomb-forge:v2-review .
```

`prepare-runtime.py` refuses a nonempty cache, downloads the exact Forge source archive and Maven 3.9.9, checks their pinned SHA-256 values, builds only the six-module desktop reactor once, and writes the exact preferences/two baseline decks. It uses a private Maven repository under the chosen cache. Public source/Maven repository connectivity and disk space for sources/dependencies/output are required only at build time. If the build fails, inspect `build.log`; do not repeatedly rebuild unrelated modules. `--plan` prints the intended downloads/command without executing them.

For a **verified existing or transferred cache**, skip preparation and run the packager directly. The packager constructs its classpath from the pinned manifest's modules/JARs under `--cache`; it does not read the old absolute Maven classpath or require the macOS desktop probe profile. It verifies bytecode/resource/dependency/archive/preferences/deck identities, then compiles only the adapter and pinned presentation extraction with `--release 17`. No Forge rebuild is needed when those inputs are unchanged. A mismatch stops packaging; inspect compiler/dependency drift rather than bypassing the check. Fresh cross-platform byte-identical builds are not assumed.

Packaging recreates output `classes`, `lib`, `res`, defaults and source directories; **use a dedicated output**, not a directory containing unrelated files. Both images are local until the owner deliberately publishes them. The named `forge-runtime` build context is required; a generic Git-only remote build does not contain it. The exact fresh-source build command is also preserved in `docs/v2/evidence/V2-010.md`; no simulation is required just to create configuration.

The bundle includes runtime resources, 89 runtime JARs, five compiled Forge modules, five fixed default decks, preference file, Java opening flags, full pinned Forge source archive, adapter/extraction/supervisor source, original license, manifest and exact per-file hashes. It does not include user deck data or service secrets. `extract_combat.py` preserves the pinned desktop `checkDamageQueue` and `getDamageToKill` method bodies, replacing only the Swing game-view poison-count access with an injected scalar. Target eligibility follows the pinned desktop view constructor. The generated source and its upstream identity are included for review; no card-specific rule is added.

## Using the usual `go run ./cmd/server` workflow

The Go command starts the web app; Forge runs separately. To use your existing local database, account and port 8080, run only the Forge container. From the repository root, first build the prepared bundle as a development image (repeat after engine/adapter changes):

```sh
docker build --platform linux/amd64 \
  --build-context forge-runtime="$FORGE_BUNDLE" \
  -f services/forge/Dockerfile -t manatomb-forge:dev .
```

Keep your existing root `.env` settings and add:

```text
CPU_PLAY_ENABLED=true
FORGE_SERVICE_URL=http://127.0.0.1:8081
FORGE_SERVICE_SECRET=<a random secret of at least 32 characters, generated once>
```

Use `openssl rand -hex 32` to generate the local secret. Keep it in the ignored `.env`; do not commit it. Both processes use that same value. Start Forge, then start (or restart) the web server:

```sh
docker compose --env-file .env -f services/forge/compose.dev.yaml up -d
go run ./cmd/server
```

Open **http://localhost:8080/cpu** and sign in with your existing local account. The default `PUBLIC_BASE_URL` is `http://localhost:8080`; use that exact browser origin, or explicitly set `PUBLIC_BASE_URL` to match your chosen origin. Changes to `.env` require restarting the Go server; exported shell variables take precedence over `.env`.

This development Compose file starts no web or database containers and binds Forge only to `127.0.0.1:8081`. Docker must be running. Check the service with `curl --fail http://127.0.0.1:8081/healthz`. Stop it with `docker compose --env-file .env -f services/forge/compose.dev.yaml stop`. An active CPU game ends when Forge is stopped. Production enablement remains disabled by default and is configured separately.

## Full packaged local topology

Set these in a **local, untracked** environment file:

```text
FORGE_RUNTIME_BUNDLE=/absolute/path/to/dedicated-bundle
FORGE_SERVICE_SECRET=<random 32+ character secret>
CPU_PLAY_ENABLED=true
```

`CPU_PLAY_ENABLED=true` is deliberate local enablement. Default is false. Start the packaged web, engine and disposable database with:

```sh
docker compose --env-file /absolute/path/to/local.env \
  -f services/forge/compose.local.yaml up --build -d
```

Open `http://127.0.0.1:18880`, create a **local test account**, then `/cpu`. The isolated database begins empty; pasted CPU/player deck lists work independently of the site card catalog. Populate a local catalog separately for builder tests. The engine has no public host port; only the web can reach `http://forge:8081`. Do not use the local PostgreSQL password or disabled TLS settings in hosting.

End the local experiment with the matching compose `down`. Add `--volumes` only when intentionally discarding this disposable local database. No global Docker prune. Existing application databases/containers are separate.

## Engine updates and new cards

Scryfall sync updates ManaTomb's searchable card catalog, images, and Oracle
text. Forge implements gameplay through its own card scripts and engine code.
A Scryfall import does not add playable cards to an already packaged Forge
runtime. Forge publishes stable releases and daily snapshots containing new
cards and fixes; see the [upstream guide](https://github.com/Card-Forge/forge/wiki/User-Guide#snapshots)
and [releases](https://github.com/Card-Forge/forge/releases).

This service currently has no automatic engine updater. Its source revision is
pinned in `prepare-runtime.py`, `package.py`, and `internal/forge/contract.go`;
the packager verifies the exact engine, resources, dependencies, and source
against `docs/v2/evidence/V2-010/manifest.json`. Merely changing the commit or
copying newer card scripts into the existing bundle will fail verification and
may introduce calls the adapter cannot handle.

For an upgrade, choose an immutable upstream commit, build it in a new cache,
check adapter/extraction compatibility, produce a new runtime manifest (keep
the old evidence), and update the source pins and packaging manifest together.
Run the Go/JavaScript checks and the real-engine regression suite below,
including saved-deck validation and representative gameplay. Then package a
new image and promote it through the private service's drain/readiness flow
after active games finish. Keep the previous image for rollback; replacing the
service during a game ends that session.

To make this routine, add a scheduled CI upgrade workflow that discovers a new
release or snapshot, resolves its commit, builds and tests a candidate, and
opens an upgrade PR with the new manifest and compatibility results. Passing
CI can prepare the image automatically; production promotion should wait for
an idle, drained engine. The current CI only tests the Go web application and
does not build or upgrade Forge, so this workflow remains to be implemented.

## Focused checks

```sh
go test -race ./internal/forge ./internal/web ./cmd/server
node --test internal/web/assets/cpu-launch.test.cjs internal/web/assets/saved_overview.test.cjs
python3 services/forge/regression.py \
  --bundle /absolute/path/to/dedicated-bundle \
  --cache /absolute/path/to/v2-010 \
  --java-home /absolute/path/to/temurin-21.0.6 \
  --output /absolute/path/to/new-regression-output
```

Repeat the last command with `--test OrderingRegression` and a distinct output path for actual controller/view ordering, or `--test ControlsRegression` for real automatic payment, floating mana, phase preferences, and combat assignments. `--test DamageTriggerRegression` reproduces the Raph & Mikey attack that reveals Old Gnawbone into combat, explicitly orders both resulting damage triggers, and verifies fourteen Treasures through the actual turn engine. `--test ActionShortcutRegression` checks deliberate single-action land, mana, and spell clicks while preserving optional and multiple choices. These are engine-backed protocol tests, not browser natural-game evidence.

## Table control contract

Each full state projection includes `players[].mana`, with six `{id, label, amount}` entries per seat, including zero counts. This is Forge's public floating pool, available outside payment prompts. The existing top-level `mana` list remains the human's offered spendable pool choices during a payment input. Forge's enabled Auto payment button is submitted as the ordinary prompt-bound `action: "ok"`; Forge chooses and activates payment sources. Disabled payment controls remain rejected. Following an explicit card click, a single playable ability proceeds directly when Forge does not mark it for confirmation. The same singleton shortcut handles Forge's subsequent additional-cost-variant check, removing the second unchanged-action menu during casting. Multiple variants, optional-cost selections, modes, targets, optional triggers, and unrelated singleton choices remain explicit. This follows the pinned desktop's presentation behavior without making gameplay decisions.

`state.phaseStops` maps `Human` and `CPU` to enabled `PhaseType` enum names. Both columns control when the **human** is offered priority during that seat's turn. The session preference action is `{request, session, revision: 0, prompt: "", action: "setPhaseStop", seat: "Human" | "CPU", phase, enabled}`. It is idempotent and never consumes or replaces a pending forced choice; its acknowledgement preserves that choice's prompt ID and revision. Preferences last for the session and initially enable every phase. The adapter supplies Forge's `isUiSetToSkipPhase` callback and `YieldUpdate.SkipPhase`; Forge decides when to skip empty-stack priority and still offers responses to the stack. A stop cannot create priority in untap or ordinary cleanup, or override an explicit Forge yield.

`state.combat` contains `{attacker, defender: {type, id}, blockers, plannedBlockers}` rows from Forge's combat view. Card references use only the current projection's public IDs; player defenders use the seat ID. Planned and declared blocks are distinct so the browser can draw different arrows. A positive card `selectable` hint highlights an offered selection; its absence does not imply that the controller will reject clicking the card.

Failures retain the existing terminal behavior and now produce a bounded reference such as `adapter_failure_assignCombat_IllegalStateException`, containing an allowlisted adapter operation and exception class. Unsupported-method/choice references remain unchanged. These references and filtered code stack frames exclude exception messages, deck contents, and raw game objects. The reported Raph & Mikey / Old Gnawbone damage-step failure was reproduced: stripping trigger annotations made two Treasure-trigger labels identical and stopped the ordering prompt. The adapter now translates the pinned damage-source, damage-target, and amount annotation only when both entities exactly match authorized projected entities. Distinct public source/amount context preserves explicit ordering and original Forge ability objects; hidden or unrecognized context remains excluded, and the ambiguous-label guard remains in place. The focused regression reaches fourteen Treasures after both creatures deal seven damage; this does not claim support for every custom-deck combat path.

## Data and lifecycle

The private protocol is `manatomb-forge/v1`. Engine requests require `Authorization: Bearer <secret>` and a trusted `X-ManaTomb-Owner` set by the web proxy. The engine must be privately routed. No database credentials belong in Forge.

Deck validation temporarily reserves capacity, avoiding concurrent expensive catalog loads. Invalid/unsupported decks terminate and reclaim their worker. A live game owns its slot until the worker is reaped. Terminal views/receipts are memory-only and bounded; service restart honestly ends sessions. Five-minute disconnect grace, fifteen-minute action-idle limit and two-hour maximum are current defaults. The no-human-prompt/worker-response watchdog defaults to 90 seconds (`FORGE_NO_PROMPT_TIMEOUT`), distinct from startup and human thinking time. This conservative limit is well above measured subsecond AI decisions, but is not a guarantee for every custom deck. Expiry stops the process, never supplies a decision.

`/healthz` is liveness; `/readyz` is admission readiness. Authenticated private `/v1/admin/drain` (POST explicit `drain` boolean) and `/v1/admin/status` (GET aggregate counts/build) support owner-operated maintenance. Shutdown closes admission before reaping any worker. No browser proxy exposes these operator routes.

There is no persistent save, public spectator, unbounded queue or multi-JVM shared game state. Keep one engine **replica**; admission is process-local across all users served by that replica.

Forge prompt/reply identity and visibility filtering remain server-side. HTTP threads never project mutable game objects. Serialized event-thread mutations have a separate synchronous reply path. Optional choices are explicit; unsupported paths fail instead of choosing an answer. ACK is admission, not successful resolution.

The worker suppresses raw per-action/game traces in production. Request receipts are limited to 128 and terminal session records to 16. Cancel/expiry sends TERM to the worker process group and escalates after three seconds. Complete-container measurements must include this supervisor, Java/native memory, file cache and transport; heap size alone is not capacity evidence.

## Notices and source

Forge and the extracted presentation constraints retain GPL-3.0-or-later notices. The pinned source and exact adapter/extraction sources travel in the bundle. Third-party JAR/resource notices remain bundled and are enumerated in `upstream-manifest.json`; card names/art/text have separate owners. A service boundary is not a claim that component obligations disappear. Before distributing a release image, include the corresponding source/build instructions and review the V2-010 component inventory. This repository has not published an image or source offer on the owner's behalf.
