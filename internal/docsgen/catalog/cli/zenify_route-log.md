---
summary: Xem các record routing model do skill ghi (số liệu plan từ znf:analyze), đếm theo site, model và gate.
---
## Khi nào dùng

Sau khoảng 30 record (vài tuần dùng cook/fix/review) để xem số route theo site và gate nào đã fire.

## Kết quả

Số route theo site (`architect`, `investigator`, `implementer`, `reviewer`, `manual`, `plan-metrics`) kèm phân bố model, số lần từng gate fire, số lần `manual` không có trigger. Dữ liệu ở `.znf/route-log` của checkout chính. Chưa có record thì in "no routes logged yet".

## Cờ

| Cờ | Ý nghĩa |
|---|---|
| `--json` | In toàn bộ record dạng JSON. |
| `--since` | Chỉ lấy record từ mốc `<n>h`, `<n>d`, `<n>w` hoặc `YYYY-MM-DD`. |

## Ví dụ

```bash
zenify route-log --since 30d
```
