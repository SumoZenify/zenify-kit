---
title: Đọc dữ liệu thật
---

# Đọc dữ liệu thật

`zenify db-read` đọc database dùng chung ở chế độ chỉ đọc, để bạn xác nhận tên collection, bảng và field trước khi viết code chạm vào chúng.

## Khi nào dùng

Dùng lệnh này trước khi viết code chạm một collection, bảng hoặc field. Tên collection và field phải lấy từ dữ liệu thật, không đoán từ tên model trong code. Skill [`/znf:ground`](/reference/skills/ground) gọi lệnh này thay bạn khi grounding một field hoặc một shape dữ liệu.

## Lệnh

```bash
zenify db-read [--profile <name>] <setup|collections|tables|doc|count|eval|sql> [arg]
```

| Lệnh con | Kết quả |
|---|---|
| `setup` | Mở wizard trong terminal để cấu hình một profile |
| `collections [chuỗi]` | Liệt kê collection Mongo, lọc theo chuỗi con nếu có |
| `tables` | Liệt kê bảng MySQL |
| `doc <collection>` | In một document thật để xem tên field |
| `count <collection>` | Đếm document |
| `eval '<biểu thức>'` | Chạy một biểu thức mongosh chỉ đọc |
| `sql '<câu lệnh>'` | Chạy một câu SQL chỉ đọc |

```bash
zenify db-read setup
zenify db-read --profile local setup
zenify db-read collections chatbot
zenify db-read --profile develop collections chatbot
zenify db-read --profile production doc chat_rooms
zenify db-read doc chat_rooms
zenify db-read sql 'DESCRIBE agents'
```

## Kết quả

Profile mặc định là `staging`. Có thể chọn `local`, `develop`, `custom` hoặc `production` bằng `--profile`; profile thiếu cấu hình sẽ dừng và không tự fallback. Git branch không tự đổi profile.

Metadata profile nằm trong `.zenify/db-read.json` ở workspace và chỉ chứa tên biến môi trường cùng tên database. Credential thật nằm trong khối `env` của `.claude/settings.local.json` hoặc environment của process. Mỗi lần chạy, lệnh in profile, engine và database đã chọn nhưng không in host hoặc credential.

`zenify db-read setup` cho chọn `staging`, `local`, `develop`, `custom` hoặc `production`, chọn MongoDB/MySQL, rồi nhập toàn bộ tham số ngay trong terminal. Dùng `--profile <name>` để bỏ qua bước chọn profile. URI Mongo và password MySQL dùng ô masked. Khi chạy lại, để trống credential sẽ giữ giá trị hiện có; credential chưa có giá trị thì setup dừng trước khi thay đổi file. Profile `production` yêu cầu xác nhận riêng và phải dùng tài khoản chỉ đọc.

`eval` và `sql` chặn các động từ ghi phổ biến để tránh thao tác nhầm. Mỗi profile phải dùng tài khoản chỉ có quyền đọc; điều này bắt buộc với `production`.

```json
{
  "profiles": {
    "staging": {
      "mongo": { "uriEnv": "MONGO_URL", "database": "app_staging" },
      "mysql": {
        "hostEnv": "MYSQL_HOST",
        "portEnv": "MYSQL_PORT",
        "userEnv": "MYSQL_USER",
        "passwordEnv": "MYSQL_PASSWORD",
        "database": "acd_staging"
      }
    },
    "production": {
      "mongo": { "uriEnv": "MONGO_URL_PRODUCTION", "database": "app_production" }
    }
  }
}
```

## Lưu ý

- Lệnh chỉ chạy được trong workspace có `.zenify/db-read.json`.
- Lần đầu cấu hình có thể chạy `zenify db-read setup`; lệnh tạo `.zenify/db-read.json` và cập nhật `.claude/settings.local.json` theo cách atomic.
- Tên collection trong database luôn ở số nhiều. Tên số ít bạn thấy trong code là tên model, không phải tên collection thật. Luôn liệt kê bằng `zenify db-read collections` trước khi dùng một tên, đừng lấy tên từ tài liệu hay từ một document mẫu.
- Khi kết nối lỗi, lệnh yêu cầu kiểm tra credential của profile và quyền truy cập mạng.
- `/znf:ground` dùng lệnh này để xác minh field trước khi code chạm vào nó. `/znf:ship` cũng đọc dữ liệu thật qua đường này khi verify một thay đổi chạm DB.

## Secret và settings.local.json

Secret dùng để đọc dữ liệu thật, ví dụ chuỗi kết nối DB hay tài khoản đăng nhập test, sống trong khối `env` của file `.claude/settings.local.json` ở workspace. File này không được commit.

`zenify db-read` đọc secret từ biến môi trường tại thời điểm chạy và không nhận credential qua tham số của chính lệnh `zenify`. URI Mongo được chuyển cho process con qua environment thay vì argv. Không dựa vào bộ lọc câu lệnh làm ranh giới bảo mật; tài khoản DB của profile phải được giới hạn quyền đọc tại server.

Cùng nguyên tắc đó áp dụng khi bạn tự viết script hay agent chạm secret: không bao giờ ghi một mật khẩu ở dạng chữ thẳng vào file, kể cả file tạm.

`zenify wt new` seed `.claude/settings.local.json` từ checkout chính vào mỗi worktree mới, qua danh sách `copy` khai trong `.claude/worktree.json`. Nhờ vậy một worktree dùng chung đúng bộ secret và đúng bộ nhớ với checkout chính, thay vì mỗi worktree tự ghi vào một bộ nhớ riêng biệt. Xem [Onboard workspace và repo](/guides/onboard-workspace) để biết `copy` còn seed những gì khác.

## Xem thêm

[`zenify db-read`](/reference/cli/zenify_db-read), [`/znf:ground`](/reference/skills/ground), [Knowledge store](/concepts/knowledge-store), [Onboard workspace và repo](/guides/onboard-workspace)

<!-- Nguồn (cho người bảo trì, không hiển thị):
- reference/cli/zenify_db-read.md (generated)
- reference/skills/ground.md (generated)
- reference/cli/zenify_wt_new.md, concepts/base-ref-and-ports.md (copy seeding settings.local.json into worktree)
-->
