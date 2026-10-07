# Agent execution guide

## Operating model

One primary agent owns an active task, integration, and `STATUS.md`. Prefer completing a small change directly. A helper is justified only by an independent bounded deliverable, such as reviewing a prompt bridge, reproducing a specific bug, or testing one default deck while the primary agent has other useful work. Default maximum: one helper at a time. Avoid a standing agent network, duplicate research, or agents polling each other.

The owner's final approval is required before merges. Prepare the code, verification evidence, and reviewable PR before requesting it. Do not enable auto-merge. If merging the deployment branch triggers production, say that in the review request and use the rollout gate. Normal reversible work within the accepted task can proceed without repeated confirmations.

## Session startup

1. Read `README.md` and `STATUS.md` in this directory, then this guide and the selected task card. Read other documents only as the task requires.
2. Inspect applicable repository instructions, working-tree changes, branch/base, and linked issue/PR state. Preserve unrelated work. Reuse a recorded evidence result unless code, dependencies, or its assumptions changed.
3. Select the first eligible task using STATUS.md's dependency rules. Verified work in `review` can support dependent work in the same working base; an unresolved owner gate cannot. If none is eligible, report the concrete gate; do not implement a future milestone to appear productive.
4. Claim the task in `STATUS.md`. Use a `codex/` branch for new implementation work unless the owner requests otherwise. Name the task ID, touched areas, expected check, and bounded deliverable before editing.
5. Finish that deliverable, run the relevant checks, self-review, and set `review` with evidence. Continue eligible related work without asking permission for each small slice; combine compatible slices into a reviewable milestone PR when practical. The owner approves integration before merge; only then mark the integrated tasks `done`. Recheck acceptance if subsequent changes invalidate earlier evidence.

The initial roadmap lives on the checkout used for planning. At V2-001, decide the implementation base from the actual approved repository state; do not assume the planning branch is the intended release branch.

## Time and token discipline

- Use one task and one immediate goal per work session. Separate a task further when it cannot produce an independently reviewable result; preserve its parent ID and acceptance requirements.
- For an uncertain spike, timebox each investigation pass to roughly 60–90 minutes of active work, with a short evidence checkpoint. These are proposed effort controls, not promised completion estimates or automatic tool timers.
- Before a third substantially similar failed approach, stop repeating it. Record the two attempts, exact blocker, new evidence, and smallest next experiment. Continue only with a materially different justified approach or an owner decision on a scope/cost change.
- Record actual token/cost usage if the runtime exposes it. Otherwise record active time and attempts; never invent token counts. No numeric token budget has been supplied.
- Cache the Forge pin, source map, build command, and resource measurements in the evidence report. Do not repeatedly clone the whole project, rescan all files, or ask multiple agents the same broad architecture question.
- Use targeted tests during a change. Run the full release suite at integration/release boundaries and repeat only when affected code or assumptions changed. Avoid snapshot tests that merely repeat the implementation.
- Do not optimize for multiple sessions, shrink all of Forge, or build a generic MTG protocol before one real human/CPU match works.

## Engineering boundaries

Forge executes card rules, game legality, turn/priority flow, resolution, combat, randomness, and CPU strategy. ManaTomb translates inputs, authenticates owners, renders the allowed view, submits human choices, and contains failures. Translate identifiers and presentation; do not add special cases to make individual cards behave differently.

Input validation, missing-card explanations, stale-command rejection, session termination, and diagnostics are required integration behavior. They are not permission to make up game outcomes. An unexpected prompt must be diagnosed; never choose the first option silently to get a game unstuck.

Do not load user-provided Forge scripts or executable content. User inputs are deck data only. Never send full hidden game state to a browser and rely on CSS to conceal it.

Before changing an accepted boundary, write the proposed decision and the evidence for it in the relevant task report. Routine module/file choices belong in the implementation PR. Paid resources, dropping a committed feature, adding a framework/service, or maintaining local Forge gameplay patches need explicit owner decisions.

## Evidence and state hygiene

Use a small task report when evidence will be reused, for example `docs/v2/evidence/V2-012.md`. This is a proposed output path, not an existing completed report. Keep raw large artifacts out of Git; link an approved artifact location or attach a compact reproduction. Do not put secrets, auth headers, private decklists, CPU hidden cards, or arbitrary raw engine dumps in public issues or reports.

Each report records: task ID, code/Forge pins, assumptions, commands and environment, observations, pass/fail per acceptance criterion, known limits, and next action. Sanitized public game logs and privileged developer diagnostic data have different audiences; the former cannot contain hidden information.

Update only the affected ledger rows and current handoff. Record blocked tasks truthfully. A skipped check is not a pass. A UI mock, source finding, or AI-vs-AI run is not proof of an interactive human game.

## Reusable assignment prompt

```text
Work on ManaTomb v2 using docs/v2/README.md, STATUS.md, AGENT_GUIDE.md,
and the selected card in TASKS.md. Execute task [ID] only after its
dependencies meet STATUS.md's evidence rules and required owner gates.
Claim it and verify the current checkout first.
Use one primary agent; delegate only a small independent subtask with a
clear benefit. Preserve Forge's authority over gameplay. Do not expand
release scope or provision paid resources. Produce the bounded deliverable,
relevant verification, a self-review, and an updated status/handoff.
Prepare changes for my review; do not merge without my explicit approval.
If blocked, provide evidence and the smallest next experiment instead of
repeatedly trying the same approach or starting a later milestone.
```

For the first implementation session, replace `[ID]` with `V2-001`. At M1, use the specific spike card rather than an instruction to “build v2.”

## Reusable helper assignment

```text
For parent task [ID], perform only [bounded subtask]. Read [specific files].
Own [files, or read-only]. Do not change STATUS.md, shared contracts, scope,
or infrastructure. Return findings/changes with evidence and remaining
uncertainty. Stop after [concrete deliverable]. The primary agent integrates.
```

## Handoff / owner-review record

```text
Task and state:
Branch / commit / PR:
User-visible outcome:
Acceptance evidence (passes, failures, unrun checks):
Files or interface changes that matter:
Forge pin / configuration, if changed:
Actual effort / attempts / usage, if available:
Known limitations and new bugs:
Owner decision needed, if any:
Next eligible task or smallest unblock action:
```

PR descriptions should lead with the problem and resulting behavior, then the checks and material limitations. Link the task ID. Use existing issue IDs when available; do not close a bundled issue such as #64 when only one item was handled. New GitHub issues/comments are published only when the owner has authorized that publication; drafting them locally is sufficient for roadmap work.
