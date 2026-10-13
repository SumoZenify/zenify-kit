<!-- Moved verbatim from subagent-driven-development/SKILL.md § Model Selection (W4 slim-skills). Read when: choosing a tier for a dispatch, or wondering why the cheapest model is not always cheapest. -->

Use the least powerful model that can handle each role to conserve cost and increase speed.

**Mechanical implementation tasks** (isolated functions, clear specs, 1-2 files): use a fast, cheap model. Most implementation tasks are mechanical when the plan is well-specified.

**Integration and judgment tasks** (multi-file coordination, pattern matching, debugging): use a standard model.

**Architecture and design tasks**: use the most capable available model.
The standalone final whole-branch review is one of these.

**Review tasks**: choose the model with the same judgment, scaled to the
diff's size, complexity, and risk. A small mechanical diff does not need the
most capable model; a subtle concurrency change does. Scoped re-reviews of
small fix diffs take a cheap-to-mid tier.

**Implementer model comes from the plan.** Use the task's `**Model:**` line (`sonnet` or `opus`);
no line → sonnet. `FAIL` — how many times the *same* test has failed for this task (ledger counts it):
`FAIL=2` → opus with the error brief on disk (the failing test, its output, the diff tried) ·
`FAIL=3` → stop dispatching implementers, run `/fix` Step 1 at ROUND 3 and come back with a
root cause.
Plans record decisions, not code, so an implementer always needs judgment: sonnet is the floor,
and haiku is no longer an implementer lane.

**Name the model on every dispatch.** `zenify up` sets
`CLAUDE_CODE_SUBAGENT_MODEL=sonnet`, so a dispatch without `model` runs on
sonnet — right for mechanical work, wrong for design judgment. Pass `'opus'`
there; naming it keeps the intent visible.

**Turn count beats token price.** Wall-clock and context cost scale with how
many turns a subagent takes, and the cheapest models routinely take 2-3× the
turns on multi-step work — costing more overall. Use a mid-tier model as the
floor for reviewers and implementers.
