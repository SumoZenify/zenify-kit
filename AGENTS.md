# AGENTS.md — zenify-kit

## 1. Read `./ARCHITECTURE.md` first

This repo has no `CLAUDE.md`. Before reading or editing code, read `./ARCHITECTURE.md`: the public-distribution invariant (the binary is public; embed no secrets or internal names) and the harness boundary (what belongs to Claude Code, what belongs to Codex). Build and test: `go build ./...` then `go test ./...`; regenerate the reference docs: `go run ./cmd/zenify docs gen`; the rules before tagging a release are in `website/changelog.md` and `.github/workflows/release.yml`.

## 2. Read the workspace-level docs

Codex does not load files from a parent directory outside the git root, so read them by hand.

- The workspace is the nearest parent directory that has a `repos/` subdirectory. From the main checkout (`repos/zenify-kit`) it is `../../`; from a worktree (`repos/zenify-kit/.worktrees/` + slug) it is `../../../../`. Check with `ls` before reading; never guess.
- At the workspace, read in this order:
  1. `CLAUDE.md` — repo routing table, shared infrastructure, git and worktree conventions.
  2. `.claude/rules/00-constitution.md` — always applies.
  3. Every rule in `.claude/rules/` whose `paths:` front-matter matches the file being touched.
  4. `.claude/memory/MEMORY.md` — the index; open the memory file that matches the work at hand.

## 3. Notes when running on Codex

- Invoke a kit skill with the `$znf:` prefix plus the skill name, e.g. `$znf:fix`, `$znf:ship`. The argument is the text after the skill name in the same message; Codex has no `$ARGUMENTS`.
- Subagents are created with Codex's subagent tool, using the same agent names as the kit: `scout`, `code-reviewer`, `ui-verifier`, `researcher`, `spec-reviewer`.
- Accepted gaps compared to Claude Code:
  - No read-guard hook: read a file before editing it; never edit a file not read in this session.
  - No Workflow tool: a skill step that needs Workflow runs sequentially, or is split across subagents.

## Code Review Rules

- No fabrication: every file, function, field, collection and endpoint name must have been read in this session; a non-obvious claim cites `file:line`.
- Do not change a shared contract (Mongo collection, MySQL table, Redis channel, BullMQ payload, HTTP between services) without sweeping all 8 repos listed in the workspace `CLAUDE.md`.
- Never merge a PR, including one the agent opened; merging is the user's deploy decision.
- Report only P0/P1 findings, each with `file:line`; no style remarks.
