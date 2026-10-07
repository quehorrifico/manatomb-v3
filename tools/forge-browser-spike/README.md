# Disposable Forge browser proof (V2-011)

Local spike only: one real `PlayerControllerHuman`, one Forge CPU, one game per Java process. This does not connect to ManaTomb accounts, save decks, or change application gameplay. Read the [evidence report](../../docs/v2/evidence/V2-011.md) for tested coverage and gaps.

## Start and stop

Requires the retained V2-010 source, compiled classes, dependencies, configuration and decks at `/Users/zeusborrego/Developer/forge/v2-010`, and Temurin 21.0.6 at the path recorded in V2-010. Do not rebuild Forge when this cache exists. The helper checks the recorded configuration hashes, compiles **only this adapter** with `javac --release 17`, then reuses V2-010's runtime command/classpath/headless flags. Full resolved arguments, adapter/deck hashes and working directory are written to `command.json` in the output folder.

```sh
cd /Users/zeusborrego/Developer/manatomb-v3
python3 tools/forge-browser-spike/run.py --scenario realistic --output /Users/zeusborrego/Developer/forge/v2-011/manual-realistic
```

Open **http://127.0.0.1:18711/** (use that exact host). Startup takes several seconds. Use a fresh output folder to retain separate transcripts. Stop with Ctrl-C in that terminal before starting another scenario. If interrupted externally, locate this specific `BrowserSpike` process with the JDK's `jps -l` and terminate its PID. Do not kill unrelated Java processes. Refresh reconnects to this same live process; restart creates a new session and invalidates old controls. There is no persistence or automatic lifecycle management.

Optional focused runs (stop the previous process first):

```sh
python3 tools/forge-browser-spike/run.py --scenario focused --output /Users/zeusborrego/Developer/forge/v2-011/manual-focused
python3 tools/forge-browser-spike/run.py --scenario blocking --output /Users/zeusborrego/Developer/forge/v2-011/manual-blocking
```

`--port` changes the local port; `--work` changes the V2-010 cache path (its retained profile still has absolute resource paths). No download, Maven build, infrastructure, database, or paid service is involved.

## Controls and realistic playthrough

1. Acknowledge any Forge-authorized setup reveal with **Submit choice**. These are Forge's warnings about cards the CPU may not play well, not cards removed from its deck.
2. Choose **Mulligan**; click the card to return in the London prompt, then **OK**, then **Keep**. The realistic game retains normal shuffled opening hands and 40 life. It uses the V2-010 Feline Ferocity (Arahbo) and Open Hostility (Saskia) Commander precons, unchanged.
3. Read the current turn, phase, priority and stack. **OK** passes one priority window; **End Turn** explicitly yields until the turn ends. Do not use End Turn when testing an intervening response window.
4. Click a card to play it, then choose the actual offered ability and submit. Forge may ask the same ability again on a different call; each new prompt requires a new choice. Select a player/card when Forge requests a target.
5. During payment, click an **untapped battlefield** land and select its mana ability, once for each required mana. Dimmed lands are tapped. Auto and Undo are disabled in this spike. No payment or optional trigger is answered automatically.
6. At an attack prompt click attackers, then OK. At a block prompt select the attacker if needed, click your blocker, then OK. Forge determines legality and combat results. Inspect the board to see selected/attacking/blocking cards.
7. For an unfamiliar unsupported prompt, preserve the local diagnostic and stop that process. Do not invent an answer to continue. General card inspection, arbitrary ordering, manual damage distribution and many other prompt families remain outside this small adapter.

The realistic scenario was exercised through opening decisions, a land play, CPU actions and trigger resolution. A full realistic precon match is **not** claimed. The independent focused corpus below proves the remaining milestone interactions and natural result.

## Repeatable focused corpus

Both fixture decks pass Forge's Commander conformance check. After the real opening decision, a **one-time start hook** places existing deck cards into a documented position using Forge APIs: human 6 Islands/6 Mountains, Goblin Piker and Storm Crow; CPU 6 Plains plus Savannah Lions (`focused`) or Siege Mastodon (`blocking`); human Bolt/Shock/Unsummon/Opt/Counterspell/Island/Mountain; CPU seven Plains; CPU 8 life. The hook is not reachable from the browser. Every subsequent action, draw, payment, transition, AI decision and result belongs to Forge. The deliberately basic-heavy decks are fixtures, not representative Commander strategy.

For `focused`, keep, yield the first CPU turn, then pass to your first main phase:

1. Cast Balmor from the command zone; pay U and R manually. Pass priority to resolve it.
2. Cast Unsummon targeting Balmor; pay U; explicitly select its cast-trigger ability when offered. Resolve the trigger and Unsummon. Choose **Yes** on Forge's commander replacement prompt. Recast Balmor: verify Forge asks **{2}{U}{R}**, pay four mana and resolve.
3. Cast Opt, pay U, select/resolve Balmor's trigger then Opt. The scry prompt shows only the permitted top card; choose Top/Bottom. Its library identity disappears after resolution.
4. Cast Lightning Bolt targeting CPU, pay R and select the Balmor trigger. **Before passing priority**, cast Shock targeting CPU and pay R/select the trigger. Observe four stack entries. Pass priority through their resolution: CPU 8 → 6 → 3. This is a response to your own spell, not proof that the CPU elected to counter a spell.
5. Advance to combat. Select Storm Crow, then OK; pass priority through combat. Its accumulated Forge buffs make the attack lethal. Observe **Winner: Human**, produced by Forge's natural game result.

For `blocking`, keep and use OK through the first CPU turn until **InputBlock**. The CPU chooses to attack with Siege Mastodon. Click Goblin Piker then OK; pass priority. Forge kills Piker and human life stays 40. Opt can be cast in the following combat-damage priority window for a shorter payment/reveal test. The final regression also continued this fixture to a natural turn-2 human win: cast Balmor in your main phase, Bolt and Shock with both triggers, then attack with the now-3-power Storm Crow.

## Boundary checks

Expand **Boundary checks**. Stale revision, invalid entity, replayed request, replayed prompt with a new request ID and invalid choice should return 409, while preserving the waiting game. Make a normal action first so replay checks have a prior request. During a valid choice select the option and use **Submit choice twice (boundary test)**: exactly one request should receive 200 and the other 409. It makes a real choice once.

Clicking a visible card at an illegal time can pass transport admission but is then rejected by Forge; the next view displays an error. A 200 admission response is not proof the engine accepted a gameplay action.

## Artifacts and boundaries

`transcript.json` contains only the human-projected DTO and sanitized event categories/choice indices, never raw Forge objects or full engine logs. It may contain the human's own cards and legitimate reveals; treat future user-deck runs accordingly. Console output is privileged developer diagnostic data and must not be published automatically. The checked-in demonstration uses only fixture/public-precon information.

This adapter depends on Forge's GPL-3.0-or-later components and retained desktop/runtime dependencies. Preserve the upstream source pin, notices and resource exceptions inventoried in V2-010 if packaging/distributing a derived work. No Forge binaries/assets or source bundle are redistributed here; no release licensing clearance is implied.

## V2-012 ability correction and ordering reproduction

The current adapter includes the owner-authorized source/context correction described in the [V2-012 follow-up](../../docs/v2/evidence/V2-012-ability-correction.md). Ability radios now show the permitted source name; descriptionless options also show that visible face's Forge card text. Existing labels in the historical instructions above predate this prefix. The original `verify_evidence.py` checks the historical adapter hash and is not evidence of a current full-corpus rerun. Use `regression.py --out /absolute/new/output` for the focused real-Forge regression; it requires the retained V2-010 cache/JDK, compiles local test/adapter classes only, and does not perform a browser playthrough.

For the capped Linux path, start the [benchmark runner](../forge-benchmark/README.md#resume-artifacts-and-focused-verification), open `http://127.0.0.1:18711/`, and follow these actual controls. Owner manual playthrough is still pending; the recorded demonstration was operated through the browser by the agent. The list intentionally describes choices, not an HTTP action script.

1. Acknowledge the authorized AI-support warning; Keep the recorded seven-card hand. End Turn through CPU turn 1. On human turn 2, OK through upkeep/draw, play Plains, End Turn. The pending 0–1 ability radio should now show Arahbo, `[Phase: ]`, and its source rules. Select that identified ability and Submit choice. Forge advances without a legal Cat target. Optionality is preserved; do not automatically submit an empty selection.
2. End Turn through CPU turn 3; acknowledge the revealed Plains from its land search. On human turn 4, play Stirring Wildwood. Cast Sol Ring with Plains; resolve it. Cast Sword of the Animist with Sol Ring's two colorless mana; resolve it. End Turn and explicitly select the identified Eminence trigger, then yield through CPU turn 5.
3. On human turn 6, cast Leonin Shikari using Plains and Stirring Wildwood, leaving Sol Ring for Equip. Select Sword's Equip ability, target Leonin, pay with Sol Ring and resolve. Enter combat with OK, select Eminence and resolve: equipped Leonin becomes 6/6. End Turn (it was just cast), then yield through CPU turn 7.
4. On human turn 8, play Terramorphic Expanse. Activate its search, explicitly confirm the sacrifice, resolve and select a Forest from the authorized library view. Enter combat, select/resolve Eminence, declare Leonin attacking. Choose Sword's attack trigger, confirm using it, select a Plains **from the library**, then pass combat priority. Forge dealt six damage in the recorded run (CPU 34). End Turn, then yield through CPU turn 9. Acknowledge the informational “CPU picked” notice; its missing player name is a recorded limitation, not a player selection supplied by the tester.
5. On human turn 10, play Rogue's Passage. Cast Arahbo using Sol Ring, Plains, Forest and Rogue's Passage (five total mana); resolve. Enter combat, select/resolve the described battlefield Eminence trigger, and attack with Leonin. **Current supported boundary:** Forge asks to order the simultaneous Arahbo/Sword triggers. Set Resolve position 1 to Sword and position 2 to Arahbo; both start blank. Submit order. Forge asks to place Arahbo first and Sword second: explicitly choose each offered attack ability. The stack now shows Sword above Arahbo. Resolve Sword, choose Yes and search for Forest. Resolve Arahbo and explicitly Cancel its optional payment (only two untapped mana sources remain). Pass combat priority and End Turn; yield through CPU turn 11 and stop at the first human turn-12 priority prompt. This is the fixed benchmark endpoint, not a natural game result.

For spells/activated abilities Forge can ask two successive `getAbilityToPlay` questions; these are distinct prompts. Select the named action again when the second real prompt appears. Mana sources also have an explicit ability radio. Use manual mana activation, not the disabled Auto button. Engine legality remains authoritative. If draws, choices or prompt families differ, inspect them and record the divergence instead of blindly following the sequence.

At the original turn-2 choice you can expand Boundary checks and test stale revision/invalid choice: both must return 409 and leave it waiting. Selecting Arahbo and using “Submit choice twice (boundary test)” must accept exactly once (one 200, one 409). Refresh should preserve the same pending choice; a new process has a different session/token. This is disposable local supervision, not production session management.

## Simultaneous-trigger ordering regression

Only the pinned `PlayerControllerHuman.orderSimultaneousSa` path is supported by `IGuiGame.order`. General card/sideboard/other ability ordering still fails explicitly. Every offered ability must occupy one position; missing or repeated selections disable submission. Position 1 is the next of these abilities to resolve, absent later stack responses. Forge returns/places this list in reverse, so placement prompts appear in the opposite order. Remember is disabled; even Forge's cached proposal must be selected again.

Run the engine-backed protocol test outside a measured session:

```sh
python3 tools/forge-browser-spike/regression.py --test ordering \
  --out /Users/zeusborrego/Developer/forge/v2-012-ordering/manual-protocol
python3 tools/forge-browser-spike/regression.py --test projection \
  --out /Users/zeusborrego/Developer/forge/v2-012-ordering/manual-projection
```

For the focused browser fixture, use the benchmark runner with a new name, `--scenario ordering --memory 1024 --timing --seconds 900`. Keep and yield the first CPU turn; advance into human combat, select/resolve Eminence, and attack with the equipped Leonin. Choose Sword in position 1 and Arahbo in position 2. Submit (or use Submit order twice for the competing-reply check). Select the reverse placement prompts, resolve Sword, confirm search and choose Forest. Resolve Arahbo and manually pay 1WG from untapped lands. Verify Leonin becomes 12/12. The fixture uses unchanged real precon cards, with a one-time setup of Arahbo, equipped Leonin and six lands; subsequent resolution belongs entirely to Forge.

The focused browser demonstration and five realistic repetitions were automated through actual controls by the agent. They are not an owner manual playthrough or proof of arbitrary Commander prompt coverage. See [ordering evidence](../../docs/v2/evidence/V2-012-ordering.md).
