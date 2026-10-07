# Sanitized V2-011 demonstration

This is a normalized transcript of **agent-operated actual browser controls**, not a recording of an independent user's playtest. Choices went through HTTP → the real Forge human controller/input machinery. No engine-side scripted player, AI human substitute, mocked reply source, or forced game result was used. Source traces remain local; this excerpt intentionally excludes hidden identities/order, full engine objects, and diagnostic logs.

| Browser operation / observation | Forge response |
| --- | --- |
| Realistic precons: acknowledge setup warning; Mulligan | `InputLondonMulligan`: return one of the new seven cards |
| Select Dreamstone Hedron, OK, Keep | Six-card human hand; CPU hand shown as count only |
| Explicit End Turn yield, then pass to human main phase | CPU played Plains and Sol Ring; subsequent human turn |
| Select Blossoming Sands; select Play land in synchronous `getAbilityToPlay` | Land moved from hand to battlefield; actual return-value call resumed |
| Select the land's offered enter trigger; pass priority | Forge resolved the stack and human life became 41 |
| Focused: cast Balmor, manually activate Island and Mountain | Forge charged `{U}{R}`, then resolved the commander |
| Cast Unsummon targeting Balmor, pay U, resolve its trigger and spell | `InputConfirm`: apply the Commander Effect replacement? |
| Choose Yes; cast Balmor again | Command zone → stack; Forge charged `{2}{U}{R}`; four browser-selected mana activations paid it |
| Cast Bolt at CPU; before resolution cast Shock at CPU | Four stack entries: Balmor trigger → Shock → Balmor trigger → Bolt |
| Pass priority through each entry | CPU life 8 → 6 → 3; Forge applied creature buffs |
| Attack with Storm Crow; pass combat priority | Natural turn-2 `Winner: Human` |
| Final regression, separate blocking fixture: Mulligan, select Mountain, OK, Keep | Real London tuck completed before the one-time fixture hook |
| Pass through CPU priority windows | CPU chose Siege Mastodon as attacker |
| Click Goblin Piker in `InputBlock`, then OK | Piker marked blocking; after damage it moved to the graveyard; human remained at 40 |
| Cast Opt, select its real ability twice, pay U through Island's ability | Both Swing-UI and game-thread synchronous waits resumed |
| Pass priority to resolve Opt | `manipulateCardList`: **Place Mountain**, with Top/Bottom; exactly one library card visible |
| Submit Bottom twice with separate request IDs through the boundary-test control | **200 accepted**, **409 Stale prompt or revision**; one choice applied; next view had no visible library card |
| Cast Balmor, Bolt and responding Shock; pass the four stack entries | CPU reached 3 life; Storm Crow became 3/2 |
| Declare Storm Crow as attacker; pass combat priority | Final adapter also reached **finished, turn 2, COMBAT_DAMAGE, Winner: Human** |

Final-version browser boundary observations:

```text
Invalid choice (-1):                    409 Invalid choice
Replay consumed request:                409 Repeated request
Replay old prompt with new request ID:  409 Stale prompt or revision
Stale revision:                         409 Stale prompt or revision
Unknown card ID (-999):                 409 Unoffered entity
Two competing valid scry replies:       200 accepted; 409 Stale prompt or revision
Cast commander during CPU combat:      admitted, then Forge rejected that action;
                                       phase, commander zone and life unchanged
```

The response above Bolt was the human responding to their own spell. CPU casting, playing lands and attacking were observed separately; a CPU counterspell or independent human usability session was not demonstrated. The fixture lowered starting CPU life to accelerate a natural result; it did not set a winner, execute turns, alter cards after initialization, or bypass Forge combat/damage rules.

The first focused scry prompt's prose incorrectly said “Place [hidden card]” because a hidden copy of a basic land censored the already authorized name. Its DTO did include the single authorized card. The correction and final browser recheck are recorded above; no initial failure is counted as a pass.
