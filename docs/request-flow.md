# Luồng request tổng quát — bản placeholder

## Phạm vi hiện tại

`POST /v2/flow/preview` là API có xác thực, nhận `{"text":"..."}` và trả NDJSON.
Đây là đường preview độc lập; `/v2/ask` chưa chuyển sang runner này.
Mọi skill trong preview đều là placeholder: không gọi nguồn, không điều khiển thiết bị,
không tải phim, không ghi Redis và không giả dữ liệu tìm kiếm.

Code: `internal/requestflow/`. `Classifier` là điểm thay thế bộ hiểu yêu cầu.
`Registry` ánh xạ tên skill sang implementation server, không tự đọc skill của Codex.

## Từ input đến response

1. Xác thực client; kiểm tra kích thước và cấu trúc input.
2. Hiểu domain, mục đích, đối tượng và URL. Mục đích quyết định nguồn cần tìm.
3. Nếu chưa rõ, trả `needs_input` cùng câu hỏi; không gọi nguồn.
4. Lập kế hoạch task và trả `plan_ready` trước khi thực thi.
5. Thực thi tối đa bốn task đồng thời; stream kết quả khi từng task hoàn thành.
6. Mỗi task giữ `source`, `task_id`, kiểu nội dung và trạng thái riêng.
7. Trả `response_completed`; lỗi một nguồn không xóa kết quả nguồn khác.
8. Chỉ thực hiện play/request/control khi có task hành động được cho phép.

Bản rule hiện tại chỉ kiểm chứng wiring bằng các mẫu câu rõ ràng. Chưa có hiểu ngữ cảnh
hội thoại, tách nhiều ý định, độ tin cậy được hiệu chuẩn hay tự sửa kế hoạch bằng AI.
Không coi regex là bộ hiểu ngôn ngữ hoàn chỉnh. Các phần này cần thay/bổ sung sau
`Classifier`, với schema được kiểm tra trước khi chạy skill.

## Định tuyến theo mục đích

| Yêu cầu | Skill được lập kế hoạch |
| --- | --- |
| Ai đạo diễn Interstellar? | movie.metadata, web.search |
| Tìm Interstellar để xem | library.search, catalog.search, video.search, web.search |
| Thêm phim vào thư viện | library.check, request.prepare |
| Tìm cách sửa lỗi Windows | documentation.search, web.search |
| Tóm tắt URL | page.extract |
| Chỉ gửi tên phim | Không chạy skill; hỏi làm rõ |
| Xin chào | answer.compose; không tìm kiếm |

Nhánh media có placeholder cho Plex, Jellyfin, Radarr, Sonarr, PhimAPI, OPhim,
NguonC, Motchill, RoPhim, Phim Tuổi Thơ, YouTube và internet.
Đây là danh sách ứng viên của kế hoạch, không phải bằng chứng nguồn đã cài hoặc online.

Khi nối adapter thật, từng task phải kiểm tra cấu hình và khả năng truy cập nguồn trước
khi tìm kiếm. Phân biệt `not_configured`, `auth_required`, `unreachable`,
`not_implemented`, `timeout`, `empty`. Không biến lỗi thành kết quả rỗng.
Tìm kiếm dịch vụ server và kiểm tra ứng dụng phát trên client là hai khả năng riêng.

## Internet: tìm trang và đọc trang

Internet là nguồn lấy dữ liệu dùng cho nhiều domain, không phải nhánh chỉ dành cho media.

- `web.search`: gọi search API phía server, mặc định dự kiến SearXNG. Nhận title,
  URL và snippet; chuẩn hóa thành `web_link`, giữ xuất xứ và thời điểm truy xuất.
- `documentation.search`: tìm theo chủ đề và ưu tiên domain chính thức; không chạy
  crawler phim chỉ vì nội dung trang có nhắc tới video.
- `page.extract`: xử lý URL được chọn hoặc nhóm kết quả cần đọc để trả lời.
  Chọn adapter theo host và loại nội dung: API → JSON nhúng → HTML → browser nếu cần.
- `movie.metadata`: lấy thông tin phim; không cần trích video để trả lời ai đạo diễn.
- `catalog.search`: tìm phim/tập; `episode.resolve` chỉ lấy stream khi chọn tập.

Để tra cứu kiến thức, planner tương lai có thể chọn một số kết quả search để đọc tiếp
rồi tổng hợp có dẫn nguồn. Đây là task phụ thuộc kết quả search, chưa có trong runner
placeholder hiện tại. Không tải toàn bộ trang của mọi kết quả một cách mặc định.

SearXNG cần bật `json` trong `search.formats`, gọi `/search?q=...&format=json`.
Cấu hình provider chưa có phải trả lỗi cấu hình, không tự sinh URL search thành nguồn dẫn.

Link web chưa chứng minh có thể phát video. Chỉ quảng bá hành động phát khi adapter
resolve hỗ trợ nguồn đó. HTML, snippet và nội dung trang là dữ liệu không tin cậy;
không được dùng chúng làm chỉ thị gọi tool hoặc thay đổi kế hoạch hệ thống.
Fetcher thật cần chặn SSRF, kiểm tra redirect/DNS, timeout và giới hạn kích thước.

## Event contract

Tất cả event có `version`, `request_id`, `sequence` tăng dần trong một request.
`task_started` phát khi task được cấp worker, không phải lúc mới xếp hàng.
`task_result` chứa kết quả riêng theo task; client cập nhật theo `task_id`.

Ví dụ minh họa phần đầu stream:

```json
{"version":1,"request_id":"r1","sequence":1,"type":"plan_ready","plan":{"understanding":{"domain":"technical","intent":"lookup","query":"Tìm cách sửa lỗi Windows","entities":{},"needs_input":false,"method":"rules"},"tasks":[{"id":"t1","skill":"documentation.search","source":"official_docs","query":"Tìm cách sửa lỗi Windows","strategy":"search_api"},{"id":"t2","skill":"web.search","source":"internet","query":"Tìm cách sửa lỗi Windows","strategy":"search_api"}]}}
{"version":1,"request_id":"r1","sequence":2,"type":"task_started","task":{"id":"t1","skill":"documentation.search","source":"official_docs","query":"Tìm cách sửa lỗi Windows","strategy":"search_api"}}
```

Placeholder trả `status: skipped`, `error_code: not_implemented`,
`cache: bypassed`, `items: []` cho từng task. Nó không trả `empty` hay dữ liệu giả.
Kết thúc toàn bộ placeholder là `unavailable`, không phải thành công.

Trạng thái kết thúc: `needs_input`, `completed`, `partial`, `unavailable`, `cancelled`.
Client ngắt kết nối sẽ hủy context; khi socket đã đóng không đảm bảo gửi được terminal event.
Mỗi skill thật phải tôn trọng context và timeout; runner giới hạn cả request ở API.

Kiểu nội dung dự kiến dùng trong `items`: `answer`, `web_link`, `media_title`,
`season`, `episode`. Hợp đồng hành động playback và trích nguồn chưa nối trong preview.

## Vị trí Redis khi triển khai thật

Trong mỗi skill đọc dữ liệu:

```text
kiểm tra cấu hình/quyền → cache lookup → adapter → chuẩn hóa → cache write → result
```

- Key gồm version schema/adapter, source, query chuẩn hóa, locale, bộ lọc và scope quyền.
- Cache hit có thể trả ngay; stale có thể trả rồi refresh theo cùng task ID, có revision.
  Cơ chế stale/revision chưa triển khai trong preview hiện tại.
- Gộp request trùng key để tránh nhiều client cùng cào một trang.
- TTL dự kiến: capability 30 giây, search 2 phút, chi tiết phim 15 phút.
- Không cache lỗi thành `empty`; không cache dữ liệu placeholder.
- Stream URL resolve lúc chọn tập; hạn cache không vượt hạn upstream.
- Không cache video, cookie/token đăng nhập hoặc response chứa bí mật.
- Redis không sẵn sàng thì bỏ cache và gọi nguồn với timeout hữu hạn.

Redis giúp request lặp lại; lần đầu vẫn chịu thời gian gọi nguồn.
Chưa dựng Redis hoặc triển khai cache backend trong thay đổi placeholder này.

## Kiểm tra

Gọi API với cơ chế xác thực hiện có và body ở trên. Không cần có Plex/website/Redis
đang chạy để kiểm tra preview. Test xác nhận định tuyến thông tin/media, kết quả đến
từng phần, không giả dữ liệu, không chạy skill khi cần hỏi rõ và một nguồn lỗi không
xóa kết quả nguồn khác.

Tài liệu provider:
- https://docs.searxng.org/dev/search_api.html
- https://docs.searxng.org/admin/settings/settings_search.html

## Live implementation update — 28/10/2026

`POST /v2/flow` now executes registered read/search skills. `/v2/flow/preview` remains the
placeholder contract described above. Input may include `context.title` and `context.sources`.
Unknown source IDs are rejected; a source restriction does not change user authorization.
Unsupported live skills return source-specific statuses; requests with no runnable skill return
`legacy_route` for the existing client path. Web chat handles both paths.

See `docs/testing/implementation-status.md` for implemented skills, environment variables,
cache behavior, remaining work and the limits of current tests.
