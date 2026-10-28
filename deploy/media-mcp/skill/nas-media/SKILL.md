---
name: nas-media
description: Tra cứu và quản lý thư viện phim trên NAS gia đình qua MCP nas-media (Plex, Sonarr, Radarr) và nas-jellyfin (Jellyfin). Dùng khi người dùng hỏi trên NAS/Plex/Jellyfin có phim gì, phim mới thêm, đang xem dở, ai đang xem, lịch tập mới, hàng đợi tải, hoặc muốn thêm phim/series vào Radarr/Sonarr.
---

# NAS media

Hai MCP server chạy trên NAS sau một proxy có token:

| Server | Dịch vụ | Quyền |
|---|---|---|
| `nas-media` | Plex, Sonarr, Radarr | Đọc; thêm phim/series và kích hoạt tìm kiếm trong *arr |
| `nas-jellyfin` | Jellyfin | Chỉ đọc |

Proxy chỉ mở các công cụ trong danh sách cho phép. Không có công cụ xóa, sửa metadata,
dọn thùng rác hay khởi động lại dịch vụ. Aria2 không có MCP; đừng hứa thao tác trên Aria2.

## Cách làm

- Hỏi thư viện: `search_media`, `get_library_items`, `get_recently_added`, `get_on_deck`
  (Plex); với Jellyfin dùng công cụ tìm kiếm và thư viện của `nas-jellyfin`.
- Ai đang xem: `get_active_sessions`.
- Tập sắp phát, phim thiếu, hàng đợi: `sonarr_get_calendar`, `*_get_missing`, `*_get_queue`.
- Kiểm tra Plex trước khi thêm vào *arr: phim có thể đã có sẵn.

## Thêm phim hoặc series

1. Tìm bằng `radarr_search` / `sonarr_search` và xác định đúng bản (tên, năm, TMDb/TVDB id).
2. Lấy profile bằng `radarr_get_profiles` / `sonarr_get_profiles`.
3. Báo người dùng tên, năm, profile sẽ dùng và **chờ đồng ý** trước khi gọi
   `radarr_add_movie` / `sonarr_add_series`.
4. Sau khi thêm, báo kết quả thật mà công cụ trả về. Nếu công cụ lỗi, nói rõ lỗi.

## Giới hạn

- Yêu cầu xóa, đổi tên, sửa metadata, dọn dẹp: giải thích là MCP chỉ cho đọc và thêm,
  người dùng tự làm trong giao diện web của dịch vụ.
- Không đoán nội dung thư viện; chỉ trả lời từ kết quả công cụ.
