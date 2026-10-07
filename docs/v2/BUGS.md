# v2 bug intake and scope

M0 snapshot refreshed **2026-09-12 07:01:29 UTC** from the repository open-issues API: **three open issues** (#64, #49, #42), one page, no PR entries or next page, no comments. Query: `state=open&per_page=100&page=1`. [Current open issues](https://github.com/quehorrifico/manatomb-v3/issues?q=is%3Aissue%20is%3Aopen). No GitHub issue/comment was published or changed.

Reproductions: app `0ab74245912407e98f2cb5131a431a67cfed0f1f`, local populated PostgreSQL, macOS in-app browser. [Full steps, expected/actual, fixtures, source evidence and limitations](evidence/V2-002.md). Task states live only in STATUS.md.

## M0 triage disposition

| Report | Local card | Classification / severity | Proposed v2 disposition and evidence |
| --- | --- | --- | --- |
| [#64 — Small bugs](https://github.com/quehorrifico/manatomb-v3/issues/64): Surtr, Fiery Jötun commander | BUG-064A | Confirmed P1, name normalization | Selection succeeds; account-save 422, saved editor says Not a Commander, explicit commander update 400. Local catalog candidate=true; Go preserves ö while SQL unaccents. Required later fix, no card-name special case. |
| #64: Kaalia of the vast commander | BUG-064A | Not reproduced; P1 if confirmed | Canonical Kaalia selection/save/update/reload works, printing retained. Not labeled fixed. Owner acceptance or precise failing deck/printing/browser needed before closing this subreport. |
| #64: autosave frequency and unsaved/signed-out state | BUG-064B | Confirmed P1 saved-note failure; P2 local/account clarity | Stopped local app, edited saved notes: UI continued Changes save automatically; restart/reload lost new note. Required recovery/acknowledgment fix before choosing frequency. Guest local persistence and failed draft-account save already have some correct behavior; slow/stale response cases unverified. |
| #64: sorting printing choices | BUG-064C | P3 enhancement | Default date ordering exists; no user-selectable printing sort in inspected picker. Defer. |
| #64: bulk import inside editor | BUG-064D | P3 enhancement | Dedicated paste/TXT/CSV import works; merge into existing editor is absent. Defer editor expansion; V2-023 setup import remains required. |
| [#49 — Document the current product flows before expanding further](https://github.com/quehorrifico/manatomb-v3/issues/49) | V2-001 | Documentation request | [Baseline/flows](evidence/V2-001.md) delivered for review; not merged or closed. |
| [#42 — Add social signals to public decks](https://github.com/quehorrifico/manatomb-v3/issues/42) | — | P3 enhancement, partially already present | Archetype tags/filter links already present and observed. Likes/favorites, view/fork counts and engagement sorting remain outside v2. Existing fork action is not a fork count. |
| Workbench document title differs from restored draft | BUG-M0-001 | Confirmed P3 presentation defect | Reload/return shows a new generated browser title while editor name/cards restore correctly; defer, no observed data loss. |

“Already present” describes verified current functionality, not a new fix or proof that an entire issue is resolved. Detailed failure coverage and unrun checks are in V2-002. #64 remains bundled and open; fixing one item must not automatically close it.

## Admission policy for newly discovered bugs

- **P0:** security compromise, private-data exposure, destructive data loss, or site outage. Stop affected rollout/work, contain the problem, notify owner, and prepare a focused patch.
- **P1:** blocks starting/completing ordinary supported CPU games, loses saved decks/drafts, leaks hidden game state, leaves unreclaimable sessions, or regresses an existing core flow. Release blocker until fixed or explicitly dispositioned by the owner; privacy/data-loss failures cannot be waved through as cosmetic limitations.
- **P2:** degraded but usable behavior with a reasonable workaround. Include if directly connected to CPU/deck setup and bounded; otherwise defer with owner-visible rationale.
- **P3 / enhancement:** presentation polish, preferences, unrelated product expansion. Schedule only inside the accepted scope and available effort; otherwise defer.

Separate **integration defects** (wrong mapping, missing prompt, lost choice, data leak, transport error, wrong rendering) from **upstream Forge defects** (card/rules/AI behavior). For upstream defects, capture the pin and minimal reproduction, link an existing upstream report if available, and explain the affected behavior. Do not create card-specific workarounds or silent outcomes. Updating the pin is a deliberate change that reruns affected compatibility/resource checks. Upstream reporting is prepared for the owner unless publication has already been authorized.

An engine exception or hang ends the affected session clearly and releases resources. Diagnostic handling is still ManaTomb's responsibility. An isolated upstream bug may be documented; widespread inability to finish representative matches fails the release gate even if the engine is the cause. The owner decides whether to postpone release, accept a specific limitation, or wait for upstream.

## Intake template

```text
Local ID / existing GitHub link:
Title and reporter-visible symptom:
Observed on app commit / Forge pin / browser:
Minimal steps and safe fixture:
Expected / actual behavior:
Reproduced? Evidence:
Severity and affected users:
Integration / upstream / data / unclear:
In v2 / defer / already fixed, with reason:
Task dependency / owner:
Regression check:
Disposition / PR / verified result:
```

Keep private decks, hidden cards and credentials out of public reports. Use a synthetic or owner-approved reproduction fixture. New bug work gets its own ledger row; do not hide it in an unrelated task's completion claim.

## Deferred ideas

Deferred scope after M0: printing sort (BUG-064C), full in-editor bulk import (BUG-064D), public-deck social expansion (#42), workbench browser-title polish (BUG-M0-001), and the future features listed in README.md. Deferred means uncommitted, not rejected forever. Reassess when planning the next release or when evidence shows an item is essential to v2's accepted behavior.

## September 23 release-review disposition

Read-only GitHub refresh still finds exactly #64, #49 and #42 open, with the same update timestamps as M0. [Sanitized snapshot](evidence/V2-M2/open-issues-sept23.json). `gh` was unavailable; the public repository API supplied the refresh. No comments, issue edits or closures were made.

Both confirmed P1 defects now have reviewable fixes and actual PostgreSQL/browser regression evidence in [V2-bug-fixes](evidence/V2-bug-fixes.md). Their original reproductions above remain historical evidence. Owner acceptance remains required before release; **Kaalia is still not reproduced and is not claimed fixed**. The optional #64 printing-sort/import items, #42 expansion and BUG-M0-001 remain deferred proposals. #49 documentation is delivered locally, unmerged.

New integration defects found during the local corpus (empty reveal reply encoding, public rules over-redaction, stale displayed card references, keyboard focus loss and hidden saved-page launcher) were corrected and linked in [V2-M2](evidence/V2-M2.md). No reproduced unresolved privacy/data-loss or unreclaimed-worker defect remains in that corpus. The current IAB download has no observed file-delivery event; the visible sanitized diagnostic preview works. This is an explicit export-delivery validation gap, not a claim that the download passed. Browser/device and native/hosted gates remain in [the final review checklist](RELEASE_CHECKLIST.md).
