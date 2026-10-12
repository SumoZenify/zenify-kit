---
summary: Dọn knowledge store - xóa plan, handoff, reference đã hết giá trị và ghi header trạng thái cho spec; mặc định chỉ in kế hoạch.
---
## Khi nào dùng

Sau khi spec đã build xong và worktree đã dọn, hoặc khi store phình ra file cũ. `zenify wt sweep` gọi lệnh này tự động sau khi dọn worktree.

## Kết quả

Lệnh tính trạng thái từng spec rồi áp bốn luật: xóa plan của spec đã built hoặc superseded, xóa handoff không ngày hoặc quá hạn, xóa reference không còn được link, và ghi lại dòng `**Status:**` trong header spec. Không có `--apply` lệnh chỉ in từng hành động và dòng tổng kết `dry-run`. Có `--apply`, mỗi file bị xóa được chép trước sang `research/` hoặc `notes/` của `docs/` (bản hạ cấp), rồi mới xóa. Lệnh không đổi state của spec và không bao giờ trả exit khác 0; store không có `specs/` thì in một dòng rồi bỏ qua.

## Cờ

| Cờ | Ý nghĩa |
|---|---|
| `--apply` | Thực hiện thay đổi. Mặc định chỉ in kế hoạch. |
| `--no-fetch` | Bỏ `git fetch` khi tính trạng thái spec, dùng ref local. |
| `--workspace` | Thư mục workspace; mặc định tự tìm, không có thì dùng thư mục hiện tại. |

## Ví dụ

```bash
zenify docs sweep
zenify docs sweep --apply --no-fetch
```

## Lưu ý

Lệnh không chạy git trong store. Thay đổi được `zenify docs sync` commit và đẩy ở lượt sau. `zenify wt sweep` gọi lệnh này tự động. Xem [Knowledge store](/concepts/knowledge-store).
