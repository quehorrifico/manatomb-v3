# Owner-operated v2 deployment guide — local review candidate

The local candidate is ready for code review; public enablement remains gated by the owner-run checks in the release checklist. The agent has not accessed a hosting account, published an image, or changed infrastructure. All steps below are for the owner after review.

## Components and cost

| Component | Initial configuration | Monthly cost |
| --- | --- | --- |
| Existing App Platform web | Keep one shared 1-vCPU / 512 MiB container | Owner screenshot: $5 |
| Existing managed PostgreSQL | Keep independent 1 GiB / 10 GiB database | Owner screenshot: $15.15 |
| New App Platform **service**, named `forge`, in the same app | One shared 1-vCPU / 2 GiB instance; no autoscaling; private HTTP only | $25 additional |

Planning total **$45.15/month**, about **+$25/month**, before tax, image-registry charges and transfer overages. The shared 2 GiB service price was checked against [official pricing](https://www.digitalocean.com/pricing/app-platform) September 23, 2026. This is the owner's selected sizing target, not proven native capacity. Preserve 20% whole-service memory headroom and the 30-second startup target in hosted validation; do not declare them passed from a single emulated run.

Use a service, not an App Platform background worker: ManaTomb needs an internal HTTP endpoint. Keep the database independent; Forge receives no database credentials. One engine replica admits **one game across the whole site**, not one per user. The next signed-in user sees busy. More users can use ordinary deck tools concurrently; additional simultaneous Forge games require new measurements and an admission design for multiple replicas.

## Build and image handoff

Follow [the pinned runtime build instructions](../../services/forge/README.md). Assemble the verified V2-010 runtime cache and compile the adapter; do not substitute an arbitrary current Forge release. Build Linux AMD64 locally:

```sh
docker build --platform linux/amd64 -t manatomb-web:v2-review .
docker build --platform linux/amd64 \
  --build-context forge-runtime=/absolute/path/to/verified-bundle \
  -f services/forge/Dockerfile -t manatomb-forge:v2-review .
```

The Forge Dockerfile uses a named local build context; a generic remote Git build will not have that cache. After review, the owner must publish the built image to a registry App Platform can read and choose its **immutable digest**. Keep the matching source bundle, manifests, Forge/license/dependency notices and build instructions with the release. Do not use a moving image tag as the release identity. Registry selection/access and any registry charge remain owner choices. Nothing here uploads an image automatically.

Web starts `/app/manatomb`, default port 8080. Forge starts `/usr/local/bin/forge-service`, default port 8081, launches the pinned Java worker itself, and needs writable ephemeral `/tmp`; no persistent game volume. The Java command fixes `-Xms64m -Xmx1024m -XX:ActiveProcessorCount=1 -XX:+ExitOnOutOfMemoryError` plus the verified module-opening flags. Do not silently raise heap to the container limit.

## Private connection and settings

Configure Forge with **internal port 8081**, no `http_port`, no public ingress route/domain. Configure its health check as HTTP `/healthz`, port 8081. App Platform documents same-app communication by component name and internal port; see [internal routing](https://docs.digitalocean.com/products/app-platform/how-to/manage-internal-routing/) and [health checks](https://docs.digitalocean.com/products/app-platform/how-to/manage-health-checks/). Private DNS/routing and the health check must be verified by the owner; local Docker networking is not provider proof.

| Variable | Web | Forge |
| --- | --- | --- |
| `PORT` | `8080` | `8081` |
| `PUBLIC_BASE_URL` | Exact public HTTPS origin, no trailing slash; keep existing correct value | Not used |
| `DATABASE_URL` | Keep managed database TLS connection | **Do not set** |
| `CPU_PLAY_ENABLED` | Initially `false`; owner explicitly enables after checks | Not used |
| `FORGE_SERVICE_URL` | `http://forge:8081` | Not used |
| `FORGE_SERVICE_SECRET` | Same new random secret, at least 32 characters, runtime secret | Same runtime secret |
| `FORGE_MAX_GAMES` | Not used | `1` |
| `FORGE_DISCONNECT_TIMEOUT` | Not used | `5m` |
| `FORGE_IDLE_TIMEOUT` | Not used | `15m` |
| `FORGE_MAX_SESSION` | Not used | `2h` |
| `FORGE_NO_PROMPT_TIMEOUT` | Not used | `90s`; ends an unresponsive worker or a continuous engine interval with no human decision. Human prompt waiting uses idle/disconnect limits instead |
| `FORGE_STARTUP_TIMEOUT` | Not used | `3m` failure deadline; **not** a replacement for the 30-second target |

Runtime paths/defaults are already in the image; do not override the launcher with user input. Keep secrets out of Git, browser settings and build arguments. The web sets the signed-in owner header; the browser never receives the engine secret or contacts port 8081.

`/healthz` verifies the supervisor is listening, not that every deck/prompt works. `/readyz` separately reports whether admission is open (503 while draining); full capacity still uses the normal busy response. Keep the platform health probe on `/healthz` so draining does not restart the service or cut off existing games. The owner smoke game below is necessary. Keep existing web `/healthz`, database TLS, migration and ordinary-site settings. v2 session state is transient memory/process state; **no new database migration** is introduced by the Forge integration or the two P1 changes. The web's existing migration/startup process still runs.

## Deployment and enablement order

1. Keep `CPU_PLAY_ENABLED=false`. Record current web image/revision and app settings for rollback. Preserve normal managed-database backup practice.
2. Owner creates the private Forge service with one 2 GiB instance and its secret; wait for health. Confirm there is no public route to it.
3. Owner deploys the reviewed web image with matching URL/secret and feature still disabled. Verify sign-in, card search, saved decks, editing and goldfish remain healthy. A missing/down Forge service must not break these flows.
4. In an owner-controlled test deployment (local Compose or separate private/staging web, not the public web with a globally enabled flag), deliberately enable CPU for the test. There is currently no per-tester allowlist: `CPU_PLAY_ENABLED=true` opens admission to every signed-in account served by that web instance. Complete acceptance before public enablement: native startup samples, final-artifact memory/headroom, a healthy site baseline versus an active game, custom decks for both seats, all default deck checks, refresh/reconnect, busy response from a second account, cancel during input/CPU work, and fresh-session recovery after service restart. Do not run a production load test without a separate deliberate plan/authorization. Use the final validation checklist's exact remaining checks.
5. Deliberately set `CPU_PLAY_ENABLED=true` only after accepting the recorded limits. Sign in, open `/cpu` or launch from a deck, finish a smoke match, confirm natural result/cleanup, then start another. Check public/private route separation and one occupied slot across two accounts.

## Updates, rollback and operational limits

Games are not saved durably. Engine restart/redeployment ends them; report that honestly. Admission is process-local, so **do not run two engine replicas or overlapping old/new workers with public admission enabled**. The private, secret-authenticated `POST /v1/admin/drain` with `{"drain":true}` closes admission while existing games remain usable. `GET /v1/admin/status` exposes only aggregate active/retained counts, cap and build identity; it contains no owner IDs or deck data. These operator routes are not proxied through the web. Send `{"drain":false}` only to reopen a healthy service, never during shutdown. Drain state is process-local and resets on restart.

Before an engine update, disable the web feature, wait until all old web instances have drained, then allow at least the disconnect grace plus in-flight request time for workers to be reclaimed (default five minutes plus 15 seconds). Confirm no active JVM/session before replacing the engine. This avoids a rolling deployment temporarily admitting two games.

For rollback, turn the flag off first. Normal site tools remain available. Revert the web image/settings to the recorded version; stop/revert the engine only after session reclamation. Keep the database independent and do not remove it or restore old data for an application-only rollback. Re-enable CPU only with a matched, reviewed web/engine protocol pair and a successful smoke test.

Monitor whole-service memory, OOM/restarts, readiness failures, action errors and cleanup. Raw game transcripts are intentionally not logged; retained receipts/session history are bounded. Temporary overload returns busy; there is no unbounded wait queue. Current local measurements are Linux AMD64 **emulated on ARM64**, with warm/unspecified storage caches. Native shared-CPU timing, provider health/proxy behavior and growth beyond one active game remain unproven.

## Exact remaining owner checks and stop conditions

Use [REVIEW_HANDOFF.md](REVIEW_HANDOFF.md) for the local manual playthrough and [RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md) for evidence. Check the candidate changelog's September 22 preparation date against the eventual release date before publication. Review does not authorize a push: merging the deployment branch can trigger deployment and is a separate owner action.

Before public access, record native Linux AMD64/CPU model, final image digests, memory limit and JVM flags. At the proposed 2 GiB/one-CPU/cap-one candidate, collect five matched starts and representative human-play samples, including a fixed-duration reached prompt hold; do not compare unlike heaps or durations. Record whole-service peak (including supervisor, cache/native charges and measurement overhead), startup, CPU/command latency, bytes and cleanup. Required proposed targets remain startup ≤30 seconds and whole-service peak ≤1.6 GiB (20% headroom). A healthy, rate-limited search/deck/save baseline versus active gameplay is still needed using realistic data. The prior 429-heavy experiment cannot satisfy this check. Do not run this load on production without a deliberate owner plan.

If these targets fail, keep CPU disabled and retain the evidence. First distinguish startup-only latency from memory growth or failed reclamation. Do not increase cap or claim the 2 GiB service is adequate. The next sizing alternative is one 4 GiB/two-shared-vCPU service ($50/month at the checked pricing, +$25 over the proposed engine), requiring a new owner cost decision and matched validation; it is not a preapproved upgrade or a demonstrated remedy.

After owner acceptance and deliberate public enablement, observe for **at least 30 minutes and completion of a smoke game**. Disable CPU immediately for hidden-state exposure, ordinary-site regression, repeated normal-match failures, OOMs or capacity that cannot be reclaimed. Keep it disabled while reverting the matched web/engine pair. Provider proxy timeouts, private routing, health probes, graceful shutdown and rolling-deploy overlap remain provider-specific checks; the local Docker results do not pass them.
