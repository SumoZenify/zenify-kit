# PR message language — machine prefix, human-language body

Read this when writing the `gh pr create` title and body, or when unsure which language each half
of a PR (and the commit) should use.

## The rule

A PR carries two audiences in one artifact. Keep them apart:

- **Title prefix — machine.** The `type(scope):` prefix is parsed by `zenify release-report`
  (`internal/release/classify.go`) to classify the changelog. It stays a conventional-commit type in
  English (`feat` / `fix` / `docs` / `perf` / `refactor` / `chore`) whatever the project's language
  is. This is the one machine token in the whole PR.

- **Description + body — human.** Everything a human reads — the description after the colon and the
  **whole body** — is a human-read artifact. Write it in the project's prose language: the same
  language its specs and plans use (see `znf:_shared/artifact-style` and the project's own
  convention). For the zenify team that is Vietnamese. Structure the body as
  **summary · what changed · how verified**, and end it with the repo's attribution line.

- **Commit messages — machine.** They are the machine record and stay English (Conventional
  Commits). The PR is the human-facing half; the commit is not.

## The failure this removes

Do not split the two halves the wrong way. A title whose description is English sitting over a body
in the project's language — or the reverse, an English body under a localized title — is the exact
inconsistency this rule exists to remove. Pick the audience per field, not per PR.

## Fixed body structure

The body is always, in this order (section headings stay exactly as written; the content is
in the project's prose language):

```
<title: type(scope): description>

## Nghiệm thu
<one line per spec SC-n when the ship-pack carries a spec path; otherwise the single line: Không có spec>

## Review còn mở
<every `minor (deferred)` / `parked` line from the SDD ledger, verbatim; otherwise the single line: Không có>

<the existing body: summary · what changed · how verified · attribution line>
```

- `## Nghiệm thu` — read the spec the ship-pack's `## Intent` names and list its `SC-n` lines.
- `## Review còn mở` — the same lines the pack's `## Deferred` carries
  (`rg -n 'minor \(deferred\)|parked —' .znf/sdd/*/progress.md`); no ledger → `Không có`.

