# 100 test case sử dụng Sentinel trên TV

Ngày nghiên cứu: 27/10/2026. Đây là corpus acceptance test đề xuất, không phải số liệu
khảo sát 100 người dùng hay bảng xếp hạng tần suất. Các tình huống đời thường được tổng hợp
từ tài liệu sản phẩm; lỗi nguồn, Redis, native playback và đăng nhập theo yêu cầu Sentinel.
Mọi ca hiện là **chưa chạy**; không có nghĩa sản phẩm đã hỗ trợ các chức năng trong bảng.

## Cơ sở nghiên cứu

Tìm kiếm và điều khiển nội dung là nhóm nhu cầu được mô tả trong tài liệu
[Apple TV](https://support.apple.com/guide/tv/search-with-siri-atvb2f98a04a/27/tvos/27)
và [Fire TV](https://developer.amazon.com/docs/fire-tv/voice-enable-your-app-and-content.html).
Hỏi thông tin ngoài media dựa vào nhóm khả năng trong
[Google Assistant](https://support.google.com/googlehome/answer/7172842?hl=en) và
[Apple TV](https://support.apple.com/en-gb/guide/tv/atvbb96a3e3f/tvos).
Các khả năng khác nhau theo thiết bị, ngôn ngữ, ứng dụng và tài khoản; đây không phải
cam kết mọi hệ điều hành đều hỗ trợ câu tiếng Việt hoặc mọi lệnh ở đây.

Tiến độ xem và track ngôn ngữ tham khảo
[Plex watch state](https://support.plex.tv/articles/201018487-mark-as-watched-or-unwatched/),
[Plex subtitles](https://support.plex.tv/articles/200471113-configuring-subtitle-support/).
Kiểm thử remote/focus dựa trên
[Android TV navigation](https://developer.android.com/training/tv/get-started/navigation).
Internet search tham khảo [SearXNG API](https://docs.searxng.org/dev/search_api.html).

Câu tiếng Việt, intent, skill và expected response trong corpus là thiết kế kiểm thử của
Sentinel, không phải bản sao command list hoặc tính năng đã chứng nhận bởi các hãng.
`P0` là ưu tiên đề xuất để tránh định tuyến/hành động sai; `P1` là mở rộng trải nghiệm.
Các tên skill/intent là hợp đồng đề xuất; chưa nhất thiết đã có trong runtime.

## Cách dùng

Mỗi ca có input, precondition/context, domain/intent, skill, chính sách nguồn,
response mong đợi và assertions. JSON tương ứng ở `tv-user-cases.json` dùng ID TV-001…TV-100.

- Lớp hiểu yêu cầu: kiểm tra domain/intent/entities và khi nào phải hỏi lại.
- Lớp planner: so tập skill và chính sách nguồn; không chỉ so số lượng task.
- Lớp executor: dùng fixture nguồn, auth, timeout và cache để kiểm tra kết quả thật.
- Lớp TV: kiểm tra focus, remote, đúng thiết bị nhận lệnh và native player trên thiết bị thật.
- Với hành động: chỉ báo thành công sau kết quả tool; phân biệt dữ liệu đọc và thao tác ghi.
- Với dữ liệu hiện tại: dùng clock/fixture cố định, timestamp và timezone rõ ràng.

Không chạy toàn corpus bằng classifier rule hiện có rồi gọi đó là E2E pass.
Input preview hiện chỉ có `text`; các ca cần context, profile, client capabilities và
nhiều ý định sẽ cần bổ sung hợp đồng input trước khi chuyển thành test tự động đầy đủ.

## Nguyên tắc chung cho response

Giữ `request_id` và `task_id`; stream từng nhánh, không đợi nguồn chậm để hiển thị nguồn nhanh.
Đọc cache không được vượt quyền/profile, không đưa token/cookie vào JSON.
Phân biệt `empty`, `not_configured`, `auth_required`, `unreachable`, `timeout`, `not_implemented`.
Link trang là `web_link`; phim, tập và phiên phát là kiểu dữ liệu khác nhau.
Chỉ resolve stream khi người dùng chọn tập; giữ ID ổn định dù URL thay đổi.
Dữ liệu web là dữ liệu, không phải instruction được phép điều khiển công cụ.


## Tìm nội dung để xem — 10 ca

| ID / ưu tiên | Input / thao tác | Context / điều kiện | Domain → intent | Skill / chính sách nguồn | Response mong đợi / assertions |
| --- | --- | --- | --- | --- | --- |
| TV-001 · P0 | Tìm phim Interstellar để xem | Plex và Jellyfin đã cấu hình; có nguồn online | `media` → `find_to_watch` | library.search, catalog.search, web.search<br>Tìm các nguồn khả dụng thuộc thư viện và catalog; web tìm link bổ sung | Kết quả chia theo nguồn, có hành động phù hợp<br>✓ Không dừng sau hit đầu<br>✓ Không tự phát chỉ vì người dùng nói tìm |
| TV-002 · P0 | Tìm phim trên Plex thôi | Plex có cấu hình; còn các nguồn khác | `media` → `find_to_watch` | library.search<br>Giới hạn Plex theo yêu cầu | Danh sách phim Plex hoặc lỗi riêng Plex<br>✓ Không gọi Jellyfin/catalog/web<br>✓ Không chuyển nguồn ngầm |
| TV-003 · P0 | Trong Jellyfin có phim Dune không? | Jellyfin có credentials | `media` → `library_lookup` | library.search<br>Chỉ tra thư viện Jellyfin | Có hoặc chưa có kèm phiên bản tìm được<br>✓ Không kết luận không có nếu nguồn lỗi |
| TV-004 · P1 | Cho xem phim hành động của Thành Long | Không có ngữ cảnh phim trước đó | `media` → `find_to_watch` | catalog.search, movie.metadata, library.search<br>Lọc theo thể loại và người tham gia; đối chiếu metadata khi cần | Danh sách đúng điều kiện<br>✓ Không coi tên diễn viên là tên phim |
| TV-005 · P1 | Có phim nào xem dưới 90 phút không? | Có metadata thời lượng | `media` → `find_to_watch` | library.search, catalog.search<br>Nguồn hỗ trợ lọc hoặc chuẩn hóa thời lượng sau tìm | Danh sách kèm thời lượng<br>✓ Không đưa phim thiếu dữ liệu như đã chắc chắn đạt điều kiện |
| TV-006 · P1 | Tìm hoạt hình phù hợp cho bé 5 tuổi | Profile trẻ em; có giới hạn nội dung | `media` → `find_to_watch` | catalog.search, movie.metadata<br>Nguồn được phép trong profile | Gợi ý có phân loại độ tuổi<br>✓ Không chỉ dựa từ khóa hoạt hình để đánh giá phù hợp |
| TV-007 · P1 | Tìm bản thuyết minh tiếng Việt của phim này | Đang xem chi tiết một title có ID | `media` → `find_to_watch` | catalog.search, library.search<br>Tra các phiên bản của title đang chọn | Các bản có audio phù hợp, ghi rõ nguồn<br>✓ Phân biệt thuyết minh với phụ đề |
| TV-008 · P1 | Phim này có bản 4K không? | Title đang chọn; client báo giới hạn codec | `media` → `find_to_watch` | catalog.search, library.search<br>Tra phiên bản cùng title | Chất lượng và khả năng phát trên thiết bị<br>✓ Không suy 4K từ tên phim<br>✓ Không hứa phát được nếu codec chưa xác minh |
| TV-009 · P0 | Tìm phim này trên tất cả nguồn đang dùng | Title rõ; chỉ ba nguồn được cấu hình | `media` → `find_to_watch` | library.search, catalog.search, web.search<br>Chỉ nguồn đã bật và khả dụng | Mỗi nguồn có trạng thái riêng và kết quả tăng dần<br>✓ Không gọi nguồn thiếu cấu hình<br>✓ Không đợi đủ nguồn mới hiển thị |
| TV-010 · P1 | Tìm trailer chính thức của phim này | Title và năm đã xác định | `media` → `find_trailer` | video.search, web.search<br>YouTube/search; ưu tiên kênh nhà phát hành | Danh sách trailer với xuất xứ<br>✓ Không trả phim đầy đủ thay trailer |

## Thông tin phim và gợi ý — 10 ca

| ID / ưu tiên | Input / thao tác | Context / điều kiện | Domain → intent | Skill / chính sách nguồn | Response mong đợi / assertions |
| --- | --- | --- | --- | --- | --- |
| TV-011 · P0 | Ai đạo diễn Interstellar? | Không có phiên phát | `media` → `lookup` | movie.metadata, web.search<br>Metadata trước; web khi thiếu hoặc cần xác minh | Tên đạo diễn có nguồn<br>✓ Không gọi Plex/*arr/resolver video |
| TV-012 · P0 | Tìm thông tin về phim Interstellar | Không có ý định phát | `media` → `lookup` | movie.metadata, web.search<br>Metadata và thông tin bổ sung | Tóm tắt ngắn, năm, người tham gia<br>✓ Không chuyển thành tìm nơi xem |
| TV-013 · P1 | Diễn viên đang xuất hiện là ai? | Chỉ biết title; không có dữ liệu cảnh hiện tại | `media` → `identify_actor` | movie.metadata<br>Đối chiếu cast; hỏi ảnh/cảnh hoặc tên nhân vật khi thiếu | Trả người đúng khi đủ bằng chứng, nếu không hỏi rõ<br>✓ Không đoán diễn viên theo vị trí phát khi không có mapping cảnh |
| TV-014 · P1 | Phim này ra mắt năm nào? | Title có ID trong context | `media` → `lookup` | movie.metadata<br>Đúng title, phân biệt remake | Năm phát hành cùng tên phiên bản<br>✓ Không dùng năm của bản cùng tên |
| TV-015 · P1 | Phim này có mấy mùa? | Đang chọn series | `media` → `lookup` | movie.metadata, catalog.detail<br>Phân biệt số mùa phát hành và số mùa nguồn đang có | Thông tin tổng và tính sẵn có<br>✓ Không coi số mùa trong thư viện là toàn bộ series |
| TV-016 · P1 | Tóm tắt phim này nhưng đừng spoil | Title rõ | `media` → `summarize` | movie.metadata, answer.compose<br>Synopsis không tiết lộ kết thúc | Tóm tắt không spoiler<br>✓ Không gọi playback<br>✓ Không tiết lộ twist |
| TV-017 · P1 | Phim này được đánh giá thế nào? | Title rõ; rating nguồn có thể khác nhau | `media` → `review` | movie.metadata, web.search<br>Tách điểm từng nguồn và nhận xét có dẫn chứng | Điểm kèm thang điểm, nguồn, nhận xét<br>✓ Không bịa điểm<br>✓ Không gộp thang điểm khác nhau thành một số |
| TV-018 · P1 | Gợi ý phim giống Interstellar | Không có profile lịch sử | `media` → `recommend` | movie.metadata, catalog.search<br>Đối chiếu chủ đề, thể loại; không cần đọc lịch sử nếu không có quyền | Gợi ý kèm lý do và tùy chọn tìm nơi xem<br>✓ Không tự phát<br>✓ Không giả đã biết sở thích cá nhân |
| TV-019 · P1 | Tối nay có gì hay để xem? | Profile có lịch sử được phép dùng | `media` → `recommend` | history.read, library.search, catalog.search<br>Ưu tiên nội dung sẵn có và chưa xem theo profile | Một số gợi ý có lý do<br>✓ Không dùng lịch sử của profile khác |
| TV-020 · P1 | Phim Dune bản nào vậy? | Có hai kết quả cùng tên khác năm | `media` → `clarify_title` | movie.metadata<br>Giữ các ứng viên cùng tên | Cho chọn theo năm/poster/đạo diễn<br>✓ Không chọn ngầm một bản |

## Series, tập và tiếp tục xem — 10 ca

| ID / ưu tiên | Input / thao tác | Context / điều kiện | Domain → intent | Skill / chính sách nguồn | Response mong đợi / assertions |
| --- | --- | --- | --- | --- | --- |
| TV-021 · P0 | Mở mùa 2 tập 3 của bộ này | Đang chọn series có ID | `media` → `play_episode` | catalog.detail, episode.resolve, player.open<br>Nguồn đang chọn, tra ID tập ổn định | Phát đúng tập bằng player native<br>✓ Không suy tập bằng nối URL |
| TV-022 · P0 | Mở danh sách tập | Đang xem chi tiết series | `media` → `browse_episodes` | catalog.detail<br>Nguồn/title đang chọn | Danh sách mùa/tập điều hướng bằng remote<br>✓ Không tải trước stream của tất cả tập |
| TV-023 · P0 | Xem tiếp phim hôm qua | Profile có hai nội dung xem dở hôm qua | `media` → `resume` | history.read<br>Lịch sử đúng profile | Cho chọn hai mục; tiếp tục vị trí đã lưu sau lựa chọn<br>✓ Không tự chọn khi mơ hồ |
| TV-024 · P0 | Phát tập tiếp theo | Đang phát tập có next ID | `media` → `next_episode` | history.read, catalog.detail, episode.resolve, player.open<br>Queue series đang hoạt động | Resolve tập kế tiếp lúc cần<br>✓ Không dùng lại URL stream tập cũ |
| TV-025 · P0 | Quay lại tập trước | Đang phát tập 4 | `media` → `previous_episode` | catalog.detail, episode.resolve, player.open<br>Thứ tự episode của nguồn | Phát tập đứng trước trong thứ tự nguồn<br>✓ Không trừ ID hay ép số tập liên tục |
| TV-026 · P1 | Mở tập đặc biệt Tết | Nguồn có tên tập đặc biệt, không có số mùa chuẩn | `media` → `play_episode` | catalog.detail, episode.resolve, player.open<br>Giữ label và source ID | Chọn đúng tập đặc biệt<br>✓ Không ép thành season 1 episode 0 |
| TV-027 · P0 | Xem lại từ đầu | Đang ở màn chi tiết một tập có tiến độ | `media` → `restart` | player.open, history.write<br>Tập đang chọn | Phát từ vị trí 0 sau hành động rõ ràng<br>✓ Không xóa lịch sử toàn series |
| TV-028 · P1 | Đánh dấu tập này đã xem | Đang chọn một tập | `media` → `mark_watched` | history.write<br>Episode ID và profile hiện tại | Cập nhật trạng thái một tập<br>✓ Không đánh dấu cả mùa |
| TV-029 · P1 | Có tập mới chưa? | Context là series đang theo dõi | `media` → `check_new_episodes` | catalog.detail, history.read<br>So tập nguồn đang có với tiến độ profile | Tập mới kèm trạng thái sẵn có<br>✓ Không coi lịch phát tương lai là tập đã xem được |
| TV-030 · P0 | Chuyển tập sau tập cuối | Đã đến tập cuối hiện có | `media` → `next_episode` | catalog.detail<br>Kiểm tra nguồn trước khi chuyển | Thông báo đã hết tập; cho quay về series<br>✓ Không lặp vô hạn hoặc phát title khác |

## Điều khiển phát và ngôn ngữ — 10 ca

| ID / ưu tiên | Input / thao tác | Context / điều kiện | Domain → intent | Skill / chính sách nguồn | Response mong đợi / assertions |
| --- | --- | --- | --- | --- | --- |
| TV-031 · P0 | Tạm dừng | Có media session hoạt động | `playback` → `pause` | player.control<br>Client đang phát | Pause và báo trạng thái thực tế<br>✓ Không tìm phim tên Tạm dừng |
| TV-032 · P0 | Phát tiếp | Một session đang paused | `playback` → `resume` | player.control<br>Session hiện tại | Tiếp tục phiên hiện tại<br>✓ Không khởi động một video mới |
| TV-033 · P0 | Tua tới phút 20 | Video có duration và hỗ trợ seek | `playback` → `seek` | player.control<br>Session hiện tại | Seek đến 1200 giây<br>✓ Không cộng thêm 20 phút vào vị trí hiện tại |
| TV-034 · P0 | Lùi lại 15 giây | Vị trí hiện tại là 8 giây | `playback` → `seek_relative` | player.control<br>Session hiện tại | Seek về 0 giây<br>✓ Không gửi timestamp âm |
| TV-035 · P1 | Chuyển tới một tiếng hai mươi phút | Video dài hơn 80 phút | `playback` → `seek` | player.control<br>Session hiện tại | Seek đến 4800 giây<br>✓ Parse đơn vị tiếng/phút đúng |
| TV-036 · P0 | Bật phụ đề tiếng Việt | Session có nhiều subtitle track | `playback` → `set_subtitle` | player.tracks<br>Track session hiện tại | Chọn vi; nếu thiếu thì báo không có<br>✓ Không tải hoặc thay track ngôn ngữ khác ngầm |
| TV-037 · P0 | Tắt phụ đề | Phụ đề đang bật | `playback` → `set_subtitle` | player.tracks<br>Session hiện tại | Tắt subtitle thực tế<br>✓ Không thay audio |
| TV-038 · P1 | Đổi sang tiếng gốc | Có metadata original language và audio tracks | `playback` → `set_audio` | player.tracks<br>Track session; hỏi lại nếu không xác định bản gốc | Đổi audio đúng ngôn ngữ<br>✓ Không tự coi tiếng Anh luôn là tiếng gốc |
| TV-039 · P1 | Họ vừa nói gì vậy? | Session hỗ trợ seek và subtitle | `playback` → `replay_dialogue` | player.control, player.tracks<br>Session hiện tại | Lùi đoạn ngắn; bật phụ đề tạm nếu hỗ trợ rồi khôi phục<br>✓ Báo giới hạn nếu player ngoài không cho điều khiển |
| TV-040 · P1 | Chuyển xuống 720p cho đỡ giật | Nguồn có rendition 720p; đang phát | `playback` → `set_quality` | player.quality<br>Session/manifest hiện tại | Đổi rendition và giữ tiến độ khi khả thi<br>✓ Không tải trang WebView<br>✓ Không hứa có 720p khi nguồn không cấp |

## YouTube, âm nhạc và truyền hình — 10 ca

| ID / ưu tiên | Input / thao tác | Context / điều kiện | Domain → intent | Skill / chính sách nguồn | Response mong đợi / assertions |
| --- | --- | --- | --- | --- | --- |
| TV-041 · P0 | Mở nhạc Sơn Tùng trên YouTube | YouTube đã cài; provider search được cấu hình | `media` → `find_music` | video.search, app.open<br>Giới hạn YouTube; phân biệt ca sĩ và bài hát | Kết quả phù hợp hoặc mở đúng nội dung theo yêu cầu<br>✓ Không gọi catalog phim |
| TV-042 · P1 | Phát bài Nơi này có anh | Không chỉ định dịch vụ; có nhiều provider | `media` → `play_music` | music.search, player.open<br>Provider nhạc được cấu hình và quyền nội dung | Chọn bài đúng; hỏi provider nếu chưa có mặc định<br>✓ Không phát bản cover tùy ý khi kết quả mơ hồ |
| TV-043 · P1 | Cho nghe nhạc không lời để ngủ | Có provider hỗ trợ playlist | `media` → `find_music` | music.search<br>Provider nhạc khả dụng | Playlist phù hợp và tùy chọn phát<br>✓ Không suy ra người dùng cho phép đặt timer |
| TV-044 · P1 | Mở karaoke bài này | Đang chọn một bài hát rõ | `media` → `find_karaoke` | video.search<br>Tìm bản karaoke theo title/người thể hiện | Kết quả karaoke đúng bài<br>✓ Không coi bản có vocal là karaoke chỉ vì cùng title |
| TV-045 · P0 | Phát video hướng dẫn nấu phở | Ý định xem hướng dẫn, không phải tìm công thức chữ | `media` → `find_video` | video.search, web.search<br>Video search và web hỗ trợ lọc video | Video có tiêu đề và nguồn<br>✓ Không gọi Plex/*arr chỉ vì yêu cầu có chữ phát |
| TV-046 · P0 | Chỉ tôi cách nấu phở | Không yêu cầu video | `general` → `how_to` | web.search, page.extract, answer.compose<br>Nguồn công thức; đọc trang chọn lọc | Hướng dẫn ngắn kèm nguồn và tùy chọn tìm video<br>✓ Không tự mở YouTube |
| TV-047 · P0 | Mở VTV1 | Có channel registry được cấu hình | `live_tv` → `play_channel` | channel.lookup, player.open<br>Registry kênh hợp lệ của người dùng | Phát đúng kênh hoặc báo chưa cấu hình<br>✓ Không tìm stream tùy ý trên web thay kênh được phép |
| TV-048 · P1 | Lịch chiếu VTV3 tối nay | Có EPG; timezone người dùng | `live_tv` → `schedule` | epg.lookup<br>EPG kênh đúng ngày địa phương | Danh sách giờ và chương trình<br>✓ Không trả lịch cũ từ cache ngoài ngày |
| TV-049 · P1 | Trận Việt Nam tối nay xem ở đâu? | Đội và môn còn mơ hồ; chưa biết giải | `sports` → `find_broadcast` | sports.lookup, web.search<br>Làm rõ trận rồi tìm đơn vị phát có quyền tại vùng | Giờ đấu và nơi xem có nguồn<br>✓ Không bịa link trực tiếp hoặc khẳng định quyền phát |
| TV-050 · P1 | Tìm video mới nhất của kênh này | Đang chọn channel ID trên YouTube | `media` → `find_video` | video.search<br>Theo channel ID và thứ tự ngày công bố | Các video mới kèm ngày<br>✓ Không dùng relevance thay cho newest |

## Ứng dụng, remote và thiết bị TV — 10 ca

| ID / ưu tiên | Input / thao tác | Context / điều kiện | Domain → intent | Skill / chính sách nguồn | Response mong đợi / assertions |
| --- | --- | --- | --- | --- | --- |
| TV-051 · P0 | Mở YouTube | Ứng dụng được cài trên TV hiện tại | `device` → `open_app` | app.open<br>Inventory ứng dụng client hiện tại | Mở đúng package và báo kết quả<br>✓ Không tìm kiếm video nếu chỉ yêu cầu mở app |
| TV-052 · P0 | Về màn hình chính | Đang ở app khác | `device` → `navigate_home` | navigation.control<br>Client nhận lệnh | Về launcher/home<br>✓ Không mở trang web có chữ home |
| TV-053 · P0 | Quay lại | Đang ở màn chi tiết sau kết quả tìm kiếm | `device` → `navigate_back` | navigation.control<br>Stack màn hình hiện tại | Về danh sách và giữ focus item cũ<br>✓ Không hủy toàn bộ hội thoại |
| TV-054 · P0 | Chọn kết quả thứ hai | Danh sách đã hiển thị có ID và thứ tự ổn định | `device` → `select_result` | navigation.select<br>Snapshot danh sách người dùng đang thấy | Chọn item thứ hai của snapshot đó<br>✓ Không đổi chỉ số khi kết quả nguồn chậm chèn thêm |
| TV-055 · P0 | Tăng âm lượng một chút | Client có volume capability | `device` → `adjust_volume` | device.volume<br>TV/session nhận lệnh | Tăng một bước theo policy thiết bị<br>✓ Không tự set 100 phần trăm |
| TV-056 · P0 | Đặt âm lượng 30 phần trăm | Thiết bị hỗ trợ volume theo tỷ lệ | `device` → `set_volume` | device.volume<br>Client hiện tại | Set 30 phần trăm hoặc báo không hỗ trợ trị tuyệt đối<br>✓ Không nhầm mức 30 thang nội bộ với 30 phần trăm |
| TV-057 · P0 | Tắt tiếng | Có thiết bị audio đang điều khiển | `device` → `mute` | device.volume<br>Audio target hiện tại | Mute và xác nhận từ kết quả tool<br>✓ Không tuyên bố đã mute khi tool lỗi |
| TV-058 · P1 | Chuyển sang HDMI 2 | TV có capability input switching | `device` → `set_input` | device.input<br>TV được ghép cặp; không phải server host | Đổi input hoặc báo giới hạn<br>✓ Không chạy lệnh trên máy server thay TV |
| TV-059 · P0 | Tắt TV sau 30 phút | TV hỗ trợ timer hoặc automation được cấu hình | `device` → `schedule_power_off` | timer.schedule, device.power<br>Đúng TV, timezone không ảnh hưởng duration | Hiện giờ dự kiến tắt và cho hủy<br>✓ Không tắt ngay |
| TV-060 · P1 | Mở chữ to hơn | Đang ở UI launcher; có cài đặt font scale | `device` → `accessibility_settings` | settings.open<br>Cài đặt UI được kiểm soát | Mở/chỉnh đúng setting; vẫn điều hướng D-pad<br>✓ Không đổi độ phân giải video |

## Tra cứu internet ngoài media — 10 ca

| ID / ưu tiên | Input / thao tác | Context / điều kiện | Domain → intent | Skill / chính sách nguồn | Response mong đợi / assertions |
| --- | --- | --- | --- | --- | --- |
| TV-061 · P0 | Thời tiết hôm nay ở Đà Nẵng | Địa điểm đã rõ | `general` → `weather` | weather.lookup<br>Provider thời tiết, không catalog media | Dự báo theo giờ địa phương và thời điểm dữ liệu<br>✓ Không dùng vị trí IP server thay địa điểm đã nói |
| TV-062 · P1 | Ngày mai có mưa không? | Chưa có vị trí được cấp quyền hoặc lưu trong context | `general` → `weather` | weather.lookup<br>Hỏi vị trí trước khi gọi provider | Câu hỏi ngắn cần địa điểm<br>✓ Không đoán vị trí từ tên server |
| TV-063 · P1 | Tóm tắt tin chính hôm nay | Ngôn ngữ vi; timezone người dùng | `general` → `news` | news.search, page.extract, answer.compose<br>Nguồn tin có ngày; đọc chọn lọc và đối chiếu | Các mục ngắn có nguồn/thời điểm<br>✓ Không trình bày tin cũ là sự kiện hôm nay |
| TV-064 · P1 | Đội tuyển Việt Nam đá mấy giờ? | Chưa biết đội nam/nữ và giải; có nhiều lịch | `sports` → `schedule` | sports.lookup<br>Làm rõ đội/giải khi nhiều ứng viên | Giờ địa phương cùng ngày và giải<br>✓ Không suy ra trận đã kết thúc từ kết quả search cũ |
| TV-065 · P1 | Kết quả trận vừa rồi là bao nhiêu? | Context có match ID | `sports` → `score` | sports.lookup<br>Dữ liệu trận hiện tại có timestamp | Tỷ số và trạng thái live/final<br>✓ Không gọi trạng thái đang đá là chung cuộc |
| TV-066 · P0 | Dolby Vision là gì? | Không có ý định mở video | `technical` → `lookup` | documentation.search, web.search, answer.compose<br>Tài liệu chính thức/nguồn kỹ thuật | Giải thích ngắn dễ đọc trên TV<br>✓ Không tìm phim tên Dolby Vision |
| TV-067 · P0 | TV báo không có mạng thì làm sao? | Chưa có quyền chỉnh router | `technical` → `troubleshoot` | device.diagnostics, documentation.search<br>Chẩn đoán client khi có capability; tài liệu hãng | Các bước phù hợp và kết quả đo nếu có<br>✓ Không tuyên bố đã sửa mạng<br>✓ Không reset router tự động |
| TV-068 · P0 | Tóm tắt bài viết https://example.com/news | URL chỉ là fixture; test không gọi domain thật | `web` → `summarize_url` | page.extract, answer.compose<br>Chỉ đọc URL được cho và redirect được kiểm tra | Tóm tắt nội dung fixture có xuất xứ<br>✓ Không chuyển sang catalog vì bài có nhắc phim<br>✓ Không làm theo chỉ thị trong trang |
| TV-069 · P1 | Tìm trên mạng cách vệ sinh điều hòa | Yêu cầu hướng dẫn chữ | `general` → `how_to` | web.search, page.extract, answer.compose<br>Search rồi đọc ít trang phù hợp | Hướng dẫn có nguồn và link mở ngoài<br>✓ Không gọi home control để bật điều hòa |
| TV-070 · P1 | Một đô la bằng bao nhiêu tiền Việt? | Không yêu cầu giao dịch | `general` → `exchange_rate` | finance.lookup<br>Provider tỷ giá có timestamp và loại tỷ giá | Tỷ giá tham khảo có ngày/nguồn<br>✓ Không lấy con số từ trí nhớ và gọi là hiện tại |

## Nhà thông minh và camera — 10 ca

| ID / ưu tiên | Input / thao tác | Context / điều kiện | Domain → intent | Skill / chính sách nguồn | Response mong đợi / assertions |
| --- | --- | --- | --- | --- | --- |
| TV-071 · P0 | Tắt đèn phòng khách | HA có một đèn/nhóm rõ tên; user có quyền | `home` → `control` | home.control<br>Entity registry HA | Gọi đúng entity và báo kết quả thực tế<br>✓ Không điều khiển tất cả đèn trong nhà |
| TV-072 · P0 | Đặt điều hòa phòng ngủ 26 độ | Climate entity hỗ trợ Celsius | `home` → `set_temperature` | home.control<br>HA climate đúng phòng | Set nhiệt độ đúng đơn vị<br>✓ Không đổi chế độ ngoài yêu cầu |
| TV-073 · P1 | Mở rèm một nửa | Cover entity hỗ trợ position | `home` → `set_position` | home.control<br>HA cover được chọn | Set vị trí mở 50 phần trăm theo convention integration<br>✓ Không đảo phần trăm đóng/mở |
| TV-074 · P0 | Mở camera trước cửa | Camera được cấu hình và user có quyền | `camera` → `view` | camera.view<br>Registry camera | Hiện stream theo capability client<br>✓ Không lấy camera bất kỳ theo từ cửa |
| TV-075 · P1 | Ai đang ở cửa? | Có camera và nhận diện không xác định danh tính | `camera` → `describe_scene` | camera.snapshot, vision.describe<br>Camera cửa được phép xem | Mô tả người/sự kiện nhìn thấy, nêu giới hạn<br>✓ Không khẳng định danh tính khi chưa có dữ liệu tin cậy |
| TV-076 · P0 | Bật chế độ riêng tư camera | Đang chọn một camera hỗ trợ privacy | `camera` → `set_privacy` | camera.control<br>Camera cụ thể | Privacy bật và xác nhận từ thiết bị<br>✓ Không chỉ ẩn hình trên TV rồi báo camera đã tắt |
| TV-077 · P1 | Trong phòng khách bao nhiêu độ? | Có temperature sensor đúng phòng | `home` → `read_sensor` | home.state<br>HA state có timestamp | Nhiệt độ và độ mới dữ liệu<br>✓ Không trả nhiệt độ thời tiết ngoài trời |
| TV-078 · P1 | Đèn phòng ngủ đang bật không? | HA entity có trạng thái unavailable | `home` → `read_state` | home.state<br>HA state | Báo chưa đọc được trạng thái<br>✓ Không coi unavailable là off |
| TV-079 · P1 | Tắt hết đèn phòng khách rồi mở phim đang xem dở | Hai hành động độc lập; có profile rõ | `mixed` → `multi_task` | home.control, history.read, player.open<br>HA entity phòng khách và lịch sử đúng profile | Kết quả riêng từng nhánh; phát đúng mục hoặc hỏi nếu nhiều<br>✓ Một nhánh lỗi không giả nhánh kia thành công |
| TV-080 · P0 | Mở khóa cửa chính | Entity hỗ trợ unlock; yêu cầu xác minh theo policy hiện có | `home` → `unlock` | home.control<br>Chỉ entity được cấp quyền; kiểm tra policy thao tác nhạy cảm | Xác minh nếu policy yêu cầu; báo kết quả tool<br>✓ Không thực hiện khi thiếu quyền<br>✓ Không mở khóa do câu nói từ TV đang phát |

## Ngữ cảnh, hội thoại và nhập liệu — 10 ca

| ID / ưu tiên | Input / thao tác | Context / điều kiện | Domain → intent | Skill / chính sách nguồn | Response mong đợi / assertions |
| --- | --- | --- | --- | --- | --- |
| TV-081 · P0 | Interstellar | Không có ngữ cảnh trước đó | `general` → `clarify` | clarification.ask<br>Chưa chọn nguồn | Hỏi muốn biết thông tin hay tìm để xem<br>✓ Không mặc định phát hoặc cào mọi nguồn |
| TV-082 · P0 | Phim đó có phụ đề Việt không? | Context duy nhất là một title đang chọn | `media` → `check_subtitles` | catalog.detail, player.tracks<br>Title/phiên bản đúng context | Trả khả năng phụ đề theo bản<br>✓ Không lẫn phụ đề giữa các nguồn |
| TV-083 · P0 | Chỉ lấy phim hài thôi | Trước đó đang hiển thị kết quả tìm phim | `media` → `refine_search` | catalog.search, library.search<br>Giữ điều kiện trước, thêm genre comedy | Danh sách được lọc và focus hợp lý<br>✓ Không bắt đầu tìm mới bỏ điều kiện trước |
| TV-084 · P0 | Không, tôi muốn bản năm 1984 | Trước đó có hai bản cùng tên | `media` → `correct_selection` | movie.metadata, navigation.select<br>Ứng viên cùng tên có năm | Chuyển đúng phiên bản<br>✓ Không giữ ID bản vừa bị phủ định |
| TV-085 · P0 | Mở cái này trên TV phòng ngủ | Đang chọn một item; hai TV online | `device` → `handoff` | device.discovery, player.open<br>Chỉ TV phòng ngủ có capability | Gửi đúng thiết bị và báo kết quả<br>✓ Không broadcast cho mọi TV |
| TV-086 · P0 | Tìm phim, à thôi, cho xem thời tiết | ASR final chứa tự sửa câu | `general` → `weather` | weather.lookup<br>Dùng ý định cuối; hỏi vị trí nếu thiếu | Bỏ task media, xử lý thời tiết<br>✓ Không chạy cả hai ý định như hai yêu cầu độc lập |
| TV-087 · P1 | tim phim hoat hinh cho be | Input gõ không dấu | `media` → `find_to_watch` | catalog.search, movie.metadata<br>Chuẩn hóa tìm kiếm, giữ nội dung gốc | Tìm hoạt hình và hỏi tuổi nếu chưa biết<br>✓ Không phụ thuộc hoàn toàn dấu tiếng Việt |
| TV-088 · P1 | Tìm phim In tơ stellar | ASR có cách đọc tên ngoại ngữ | `media` → `find_to_watch` | entity.resolve, catalog.search<br>Fuzzy match có ứng viên xác nhận khi chưa chắc | Đề xuất title phù hợp<br>✓ Không tự nhận fuzzy match là chắc chắn |
| TV-089 · P0 | Bấm D-pad xuống rồi Back | UI đang nhận kết quả nguồn chậm | `device` → `navigate` | navigation.control<br>Snapshot UI hiện tại | Focus không nhảy khi thêm kết quả; Back về màn trước<br>✓ Không cần chuột/cảm ứng<br>✓ Không focus vào item đã bị gỡ |
| TV-090 · P1 | Tìm phim Dune và cho biết ai đạo diễn | Một input có hai mục đích cùng entity | `mixed` → `multi_task` | library.search, catalog.search, movie.metadata<br>Tìm nơi xem và metadata dùng chung entity | Hai vùng thông tin rõ, kết quả từng phần<br>✓ Không coi hỏi đạo diễn là lệnh phát |

## Nguồn lỗi, đăng nhập và phục hồi — 10 ca

| ID / ưu tiên | Input / thao tác | Context / điều kiện | Domain → intent | Skill / chính sách nguồn | Response mong đợi / assertions |
| --- | --- | --- | --- | --- | --- |
| TV-091 · P0 | Tìm phim khi Plex đang offline | Jellyfin vẫn hoạt động; Plex probe timeout | `media` → `find_to_watch` | library.search, catalog.search<br>Cô lập lỗi Plex, tiếp tục nguồn khác | Có kết quả Jellyfin và trạng thái Plex<br>✓ Không kết luận thư viện rỗng từ timeout |
| TV-092 · P0 | Tìm phim khi một website trả rất chậm | Nguồn A trả ngay, B chậm hơn deadline | `media` → `find_to_watch` | catalog.search<br>Deadline từng nguồn và toàn request | A hiển thị trước; B timeout riêng<br>✓ Không chặn kết quả A đến khi B xong |
| TV-093 · P0 | Tìm lại cùng phim khi Redis bị ngắt | Cache backend không truy cập được | `media` → `find_to_watch` | library.search, catalog.search<br>Bỏ cache, giữ giới hạn request nguồn | Vẫn có kết quả hoặc lỗi nguồn thực tế<br>✓ Không biến lỗi Redis thành lỗi không có phim |
| TV-094 · P0 | Đăng nhập engine đang chọn | Server ở máy A đã có native session hợp lệ; TV là B | `account` → `login` | engine.auth_status<br>Kiểm tra native session trên A | Hiện đã đăng nhập, không mở flow mới<br>✓ Không yêu cầu client B đăng nhập lại |
| TV-095 · P0 | Đăng nhập engine trên TV | Server A chưa có session; provider hỗ trợ device/browser auth | `account` → `login` | engine.auth_start, engine.auth_status<br>Flow theo provider/method trên A | TV hiện URL/mã/QR; chỉ thành công sau xác minh<br>✓ Không mở terminal trên B thay A<br>✓ Không coi process exit 0 là đã login |
| TV-096 · P0 | Phát tập bằng VLC nhưng TV chưa cài VLC | Client inventory có system player hỗ trợ MIME | `playback` → `open_native` | player.capabilities, player.open<br>Ưu tiên VLC nếu có, fallback player native có hỗ trợ | Mở system player hoặc báo thiếu player<br>✓ Không mở WebView âm thầm |
| TV-097 · P0 | Phát tiếp khi URL stream hết hạn | Episode ID ổn định; lưu tiến độ 600 giây | `playback` → `refresh_stream` | episode.resolve, player.open<br>Resolve URL mới cho cùng episode, retry hữu hạn | Tiếp tục gần vị trí cũ<br>✓ Không dùng TTL Redis làm bằng chứng URL còn sống |
| TV-098 · P0 | Hủy tìm kiếm để tìm phim khác | Request cũ đang stream; request mới có ID khác | `general` → `cancel_request` | request.cancel<br>Hủy context cũ, tách request ID | UI chỉ cập nhật request đang hoạt động<br>✓ Không để kết quả muộn ghi đè tìm kiếm mới |
| TV-099 · P0 | Tìm phim trên dịch vụ chưa đăng nhập | Nguồn trả 401; nguồn khác bình thường | `media` → `find_to_watch` | catalog.search, account.status<br>Nguồn lỗi báo auth_required riêng | Nút đăng nhập theo nguồn và kết quả nguồn khác<br>✓ Không cache 401 thành empty<br>✓ Không lộ API key trong response |
| TV-100 · P0 | Dừng giữa chừng rồi mở lại app | Player native đã lưu tiến độ; token stream cũ hết hạn | `media` → `resume` | history.read, episode.resolve, player.open<br>Profile và episode ID ổn định | Resolve mới rồi resume; quay lại giữ focus<br>✓ Không yêu cầu tải lại trang web nguồn<br>✓ Không trộn tiến độ profile khác |

## Nhận xét cho kiến trúc hiện tại

1. Không thể dùng một `media.search` cho mọi câu có tên phim: lookup, recommendation, playback và request là các ý định khác nhau.
2. Internet phục vụ media lẫn thông tin chung; tìm link và đọc nội dung cần là hai bước riêng.
3. Cần context có cấu trúc: title/episode đang chọn, phiên phát, snapshot kết quả, profile và capability client.
4. Cần phân biệt task đọc với hành động ghi, đặc biệt thêm thư viện, đặt timer và điều khiển thiết bị.
5. Cần nhiều intent trong cùng request và correction/cancel qua nhiều lượt; regex hiện tại chưa đáp ứng.
6. Redis là tối ưu cho truy xuất lặp lại; test đúng vẫn cần pass khi cache tắt hoặc lỗi.
7. Bước tiếp theo nên chọn một tập P0 nhỏ triển khai xuyên suốt trước, giữ 100 ca làm acceptance backlog.
