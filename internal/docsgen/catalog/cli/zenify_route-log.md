---
summary: Show model-routing records written by skills (plan metrics from znf:analyze), counted by site, model and gate.
---
## When to use

After a few weeks of cook/fix/review use, to see how many routing records were written, by which site and model, and which gates fired.

## Output

Route count per site (`architect`, `investigator`, `implementer`, `reviewer`, `manual`, `plan-metrics`) with the model distribution, how many times each gate fired, and how many `manual` records carry no trigger. Data lives in `.znf/route-log` of the main checkout. With no records it prints "no routes logged yet".

## Flags

| Flag | Meaning |
|---|---|
| `--json` | Print every record as JSON. |
| `--since` | Keep records from `<n>h`, `<n>d`, `<n>w` or `YYYY-MM-DD` onward. |

## Example

```bash
zenify route-log --since 30d
```
