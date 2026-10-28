# 80 kịch bản mở rộng smarthome và camera

Bổ sung cho 100 ca TV trong `tv-user-cases.md`. ID riêng HC-001…HC-080 để không đổi
corpus cũ. Ngày tổng hợp: 27/10/2026. Đây là acceptance backlog **chưa chạy**, không phải
80 tính năng đã triển khai hay danh sách automation đang được bật trong nhà.

## Cơ sở và ranh giới

Cấu trúc trigger → condition → action dựa trên
[Home Assistant triggers](https://www.home-assistant.io/docs/automation/trigger/).
Các lựa chọn xử lý lần chạy chồng nhau dựa trên
[Home Assistant modes](https://www.home-assistant.io/docs/automation/modes/).
[Person integration](https://www.home-assistant.io/integrations/person/) là một nguồn trạng
thái người; occupancy theo vùng camera là dữ liệu khác. Không đồng nhất headcount với danh tính.
Các tình huống cụ thể, policy đề xuất và tiêu chí chấp nhận là thiết kế của Sentinel.

HA sở hữu trạng thái thiết bị và automation. Sentinel hiểu yêu cầu, trình bày dữ liệu,
gọi tool rõ ràng và phối hợp những rule đã cấu hình. Không mặc định dựng một nguồn trạng thái
thiết bị thứ hai hoặc tự triển khai automation mới. Với rule đang nằm trong Sentinel,
cần làm rõ ownership trong thiết kế tiếp theo thay vì tạo thêm bản sao rule ở HA.

Camera/vision/audio chỉ phát quan sát và kết quả xử lý. Muốn biến chúng thành hành động phải
có rule, điều kiện, target, receiver và quyền đã cấu hình. Tài liệu này không kích hoạt
bất kỳ điều khiển, truyền lời qua loa hoặc gửi thông báo nào.

## Những điểm bắt buộc trong hợp đồng dữ liệu

- Event: event_id, source_id, zone_id/entity_id, event_type, source timestamp, received_at,
  freshness và confidence khi có; cần thêm version/sequence nếu producer hỗ trợ.
- Dữ liệu người: tách identity theo tracker khỏi observed occupancy/headcount.
- Trạng thái: occupied, observed_empty, unknown/stale; mất stream không đồng nghĩa vắng người.
- Quyền: profile, entity/camera scope, receiver và loại action được cho phép.
- Hành động: action_id, trigger/rule ID, target, deadline, deduplication key và actual result.
- Camera stream: session/resource token, thời hạn, quyền hiện tại và khả năng thu hồi.
- Trace: liên kết observation → condition → decision → tool result để giải thích được.

Đây là hợp đồng đề xuất, chưa phải schema event hiện đã có. Worker vision hiện có
zone_id, event_type và timestamp, có thể kèm metadata; chưa bảo đảm event_id hay delivery
idempotency. Debounce mặc định được ghi trong camera AGENTS: enter 3 giây, leave 60 giây;
case thật phải dùng cấu hình thực tế thay vì hardcode các số này.
YAMNet hiện là bộ phân loại âm thanh, không phải hệ xác minh nguồn phát hay danh tính.

## Cách kiểm chứng

Mỗi ca có trigger/input, preconditions, domain/intent, skill đề xuất, outcome, assertions và
authorization. Tên skill là hợp đồng dự kiến, không khẳng định runtime đã đăng ký.
P0/P1 là ưu tiên kỹ thuật đề xuất; không phải thống kê tần suất hay mức an ninh đã chứng nhận.

Dùng fake clock và fixture state/event để kiểm tra debounce, quiet hours, timeout, ordering.
Dùng mock HA/camera/audio cho action và lỗi; chỉ test live khi người dùng đã cho phép đúng
thiết bị và thao tác. Không chạy hàng loạt corpus này trực tiếp lên khóa cửa, rèm hoặc ổ cắm.
Cảnh báo sensor/âm thanh không phải kết luận chắc chắn về sự cố; kết quả phải nói rõ nguồn
và độ chắc chắn, không giả đã xử lý tình huống chỉ vì notification gửi thành công.


## Điều khiển và trạng thái smarthome — 10 ca

| ID / ưu tiên | Trigger / input | Điều kiện | Domain → intent / skill | Kết quả và tiêu chí | Quyền / ủy quyền |
| --- | --- | --- | --- | --- | --- |
| HC-001 · P0 | Bật đèn chỗ tôi đang ngồi | TV gắn với phòng khách; không có định vị chính xác ghế | `home` → `resolve_target`<br>home.entity_resolve, home.control | Dùng phòng đã gán nếu duy nhất; nếu còn mơ hồ thì hỏi<br>✓ Không suy vị trí người từ vị trí server<br>✓ Không bật đèn phòng khác | Quyền control và ngữ cảnh phòng được phép |
| HC-002 · P0 | Tắt tất cả đèn tầng một | HA có area/floor và nhiều loại thiết bị | `home` → `area_control`<br>home.entity_resolve, home.control | Chọn đúng tập light thuộc tầng một, báo thành công/thất bại theo thiết bị<br>✓ Không tắt ổ cắm/khóa cửa<br>✓ Không báo toàn bộ thành công khi một entity lỗi | Quyền control từng entity |
| HC-003 · P0 | Giảm đèn xuống còn 20 phần trăm | Đèn đang chọn chỉ hỗ trợ on/off | `home` → `set_brightness`<br>home.capabilities, home.control | Báo không hỗ trợ chỉnh sáng; giữ trạng thái hiện tại<br>✓ Không coi bật/tắt liên tục là dim<br>✓ Không giả đã đặt 20 phần trăm | Quyền đọc capability, control khi hỗ trợ |
| HC-004 · P1 | Đổi đèn sang vàng ấm | Đèn hỗ trợ nhiệt độ màu, không RGB | `home` → `set_color_temperature`<br>home.capabilities, home.control | Đặt mức ấm trong dải nhiệt độ màu được hỗ trợ<br>✓ Không gửi RGB ngoài capability<br>✓ Không thay độ sáng ngoài yêu cầu | Quyền control |
| HC-005 · P0 | Bật điều hòa 26 độ rồi giảm quạt | Climate chỉ hỗ trợ một số fan mode | `home` → `multi_property_control`<br>home.capabilities, home.control | Đặt nhiệt độ và chọn fan mode hợp lệ; hỏi nếu giảm không xác định được<br>✓ Tách kết quả từng thuộc tính<br>✓ Không gửi mode tự tạo | Quyền climate control |
| HC-006 · P1 | Đóng rèm nhưng chừa một khe | Không có mức phần trăm đã lưu cho một khe | `home` → `clarify_position`<br>clarification.ask | Hỏi mức mở mong muốn hoặc đề xuất mức để người dùng chọn<br>✓ Không tự gán vị trí khi không có preset<br>✓ Không đảo nghĩa phần trăm mở | Chưa thực hiện control khi cần làm rõ |
| HC-007 · P0 | Tắt ổ cắm máy lọc nước | Có hai entity trùng tên ở hai phòng | `home` → `clarify_target`<br>home.entity_resolve, clarification.ask | Cho chọn đúng phòng/thiết bị trước khi tắt<br>✓ Không chọn theo thứ tự entity<br>✓ Không gửi cả hai | Quyền control sau khi chọn rõ |
| HC-008 · P0 | Kiểm tra cửa ban công đã đóng chưa | Có contact sensor và cover cùng vị trí | `home` → `read_state`<br>home.state | Đọc sensor phù hợp, hiển thị timestamp và trạng thái<br>✓ Không đồng nhất trạng thái motor stopped với cửa đã đóng | Quyền đọc sensor |
| HC-009 · P0 | Tôi vừa tắt bằng công tắc tường | Thiết bị gửi state mới sau hành động tay | `home` → `sync_state`<br>home.state | UI cập nhật state mới từ HA<br>✓ Không dùng cache cũ ghi đè thao tác tay<br>✓ Không tự bật lại vì mục tiêu cũ | Quyền đọc state; automation override theo policy |
| HC-010 · P1 | Thiết bị nào trong phòng đang bật? | HA area rõ; một số entity unavailable | `home` → `list_active_devices`<br>home.state | Danh sách đang bật và nhóm chưa đọc được trạng thái<br>✓ Không coi unavailable là off<br>✓ Không liệt kê entity không được phép đọc | Quyền đọc theo entity/area |

## Ngữ cảnh sinh hoạt và lịch hẹn — 10 ca

| ID / ưu tiên | Trigger / input | Điều kiện | Domain → intent / skill | Kết quả và tiêu chí | Quyền / ủy quyền |
| --- | --- | --- | --- | --- | --- |
| HC-011 · P0 | Bật chế độ xem phim | Scene đã tồn tại, có đèn/rèm, không có lệnh mở khóa | `home` → `activate_scene`<br>home.scene | Gọi scene rõ tên; đọc lại kết quả nếu có hỗ trợ<br>✓ Không tự sáng tác hành động scene<br>✓ Không coi HTTP accepted là mọi thiết bị đã đạt state | Quyền kích hoạt scene đã cấu hình |
| HC-012 · P1 | Khi phim tạm dừng thì bật đèn nhẹ | Chưa có rule tương ứng | `automation` → `prepare_rule`<br>automation.prepare | Hiện bản rule trigger player paused, conditions và đèn đích; lưu khi user chọn<br>✓ Không tự triển khai rule còn thiếu entity<br>✓ Không dùng sự kiện buffer như pause chủ động | Quyền tạo automation; cần đích và thông số rõ |
| HC-013 · P1 | Khi xem tiếp thì trả đèn như cũ | Đèn có snapshot trước khi pause; có thay đổi tay sau đó | `automation` → `restore_scene_state`<br>automation.evaluate, home.control | Chỉ khôi phục theo policy có xét thay đổi tay<br>✓ Không ghi đè chỉnh tay mới hơn<br>✓ Snapshot theo phiên phim | Rule đã được bật trước đó |
| HC-014 · P0 | Tắt quạt sau 20 phút | Một quạt rõ; scheduler đã cấu hình | `home` → `schedule_action`<br>timer.schedule | Tạo lịch một lần gắn target và trả thời điểm dự kiến<br>✓ Không tắt ngay<br>✓ Không tạo lặp hàng ngày | Quyền control và lập lịch |
| HC-015 · P0 | Hủy lịch tắt quạt vừa đặt | Có timer ID trong context | `home` → `cancel_scheduled_action`<br>timer.cancel | Hủy đúng timer và phản hồi trạng thái hủy<br>✓ Không hủy lịch của thiết bị/profile khác<br>✓ Không báo hủy nếu action đã xảy ra | Quyền sửa timer của người dùng |
| HC-016 · P1 | Mỗi tối 10 giờ tắt đèn sân | Timezone và target rõ | `automation` → `prepare_schedule`<br>automation.prepare | Rule lịch hằng ngày có timezone, target, enabled state rõ<br>✓ Không dùng timezone của server thay user<br>✓ Không tạo hai rule trùng khi bấm lại | Quyền tạo automation |
| HC-017 · P0 | Chúc ngủ ngon | Có scene ngủ đã cấu hình; chưa biết tên này là chào hay lệnh | `home` → `resolve_scene_alias`<br>home.scene, clarification.ask | Thực hiện scene khi alias rõ trong context; nếu không thì hỏi<br>✓ Không tắt điện cả nhà từ câu chào chung | Alias/scene được người dùng cấu hình |
| HC-018 · P0 | Tôi ra khỏi nhà | Có rule away; một người khác vẫn home | `home` → `prepare_away`<br>home.state, automation.evaluate | Đánh giá occupancy cả nhà trước các hành động phụ thuộc vắng người<br>✓ Không tắt thiết bị đang phục vụ người còn ở<br>✓ Không suy whole-home away từ một người | Rule away đã bật; quyền theo action |
| HC-019 · P0 | Tôi về rồi | Tín hiệu user hợp lệ; có scene chào và giờ yên tĩnh | `home` → `arrival_scene`<br>automation.evaluate, home.scene | Chào bằng UI hoặc hành động theo giờ; tránh phát loa nếu policy quiet<br>✓ Không phát TTS lớn mặc định<br>✓ Không mở khóa tự động | Rule arrival đã cấu hình |
| HC-020 · P1 | Tạm ngừng tự tắt đèn trong một giờ | Rule chỉ áp dụng phòng khách | `automation` → `temporary_override`<br>automation.override | Override có hạn, hiển thị thời điểm hết hạn<br>✓ Không disable vĩnh viễn<br>✓ Không tắt các automation phòng khác | Quyền chỉnh rule đích |

## Hiện diện theo phòng và vùng camera — 10 ca

| ID / ưu tiên | Trigger / input | Điều kiện | Domain → intent / skill | Kết quả và tiêu chí | Quyền / ủy quyền |
| --- | --- | --- | --- | --- | --- |
| HC-021 · P0 | Camera thấy người đi vào vùng sofa | Person tồn tại liên tục đủ enter debounce đã cấu hình | `presence` → `zone_enter`<br>vision.presence, event.publish | Phát enter cho zone một lần, kèm thời điểm/nguồn<br>✓ Không phát enter lại mỗi frame<br>✓ Không suy ra danh tính | Nguồn camera và receiver đã cấu hình |
| HC-022 · P0 | Người đi ngang sofa chưa đủ debounce | Person detection chỉ kéo dài thời gian ngắn | `presence` → `debounce_enter`<br>vision.presence | Chưa phát occupancy enter<br>✓ Không kích hoạt scene chỉ từ một frame | Quan sát camera được phép |
| HC-023 · P0 | Người rời sofa rồi quay lại trước hạn leave | Đang occupied; chưa đủ leave debounce | `presence` → `debounce_leave`<br>vision.presence | Giữ occupied, hủy pending leave khi trở lại<br>✓ Không tắt TV/đèn trong khoảng trống ngắn | Rule rời vùng đã cấu hình nếu có |
| HC-024 · P0 | Không còn ai trong vùng đủ leave debounce | Stream vẫn khỏe; dữ liệu liên tục và mới | `presence` → `zone_leave`<br>vision.presence, event.publish | Phát leave sau ngưỡng; chuyển observed_empty<br>✓ Không suy cả nhà trống từ một zone | Receiver được cấu hình |
| HC-025 · P0 | Camera bị che khi đang có người | Mất khả năng quan sát; không có sensor bổ sung | `presence` → `observation_unavailable`<br>vision.health, presence.fuse | Trạng thái unknown/stale, không phát kết luận vắng<br>✓ Không biến thiếu khung hình thành leave | Quyền đọc camera health |
| HC-026 · P0 | Hai người vào, một người rời phòng | Headcount còn ít nhất một người, vùng vẫn quan sát được | `presence` → `occupancy_update`<br>vision.presence | Giữ occupied, cập nhật count có độ tin cậy<br>✓ Không phát empty khi chỉ giảm số người | Quan sát zone được phép |
| HC-027 · P1 | Người ngồi yên lâu xem phim | Motion pre-filter ít thay đổi nhưng có occupancy đã xác nhận | `presence` → `maintain_occupancy`<br>vision.presence, presence.fuse | Duy trì/revalidate theo thiết kế không coi bất động là vắng<br>✓ Không tắt đèn chỉ vì motion bằng không | Rule occupancy đã cấu hình |
| HC-028 · P0 | Mèo nhảy lên sofa | Detector xác định animal; không có person | `presence` → `ignore_non_person`<br>vision.presence | Không tạo person enter<br>✓ Không bật scene người ngồi sofa vì vật nuôi | Quan sát zone được phép |
| HC-029 · P1 | Người đứng sát ranh giới hai vùng | Bounding box dao động qua biên; policy vùng có cấu hình | `presence` → `zone_boundary`<br>vision.presence | Áp dụng policy vùng và debounce ổn định<br>✓ Không spam enter/leave liên tục<br>✓ Không đếm một người hai lần nếu yêu cầu loại trùng | Zone policy đã chốt |
| HC-030 · P0 | Điện thoại away nhưng camera vẫn thấy người | Phone tracker mới báo away, camera có person mới | `presence` → `conflicting_evidence`<br>presence.fuse, home.state | Giữ tách trạng thái người theo tracker và occupancy phòng; thể hiện xung đột<br>✓ Không ghi đè person identity từ headcount<br>✓ Không kích hoạt all-away ngầm | Các nguồn presence được cấp quyền |

## Xem và điều khiển camera — 10 ca

| ID / ưu tiên | Trigger / input | Điều kiện | Domain → intent / skill | Kết quả và tiêu chí | Quyền / ủy quyền |
| --- | --- | --- | --- | --- | --- |
| HC-031 · P0 | Mở camera sân lên TV phòng khách | Hai TV online, camera rõ tên | `camera` → `view_on_device`<br>camera.resolve, device.resolve, camera.view | Chỉ TV đích nhận phiên xem<br>✓ Không broadcast toàn bộ TV<br>✓ Không dùng web client làm đích mặc định | Quyền xem camera và gửi TV đích |
| HC-032 · P1 | Cho xem bốn camera cùng lúc | TV báo giới hạn decoder; có substream | `camera` → `multiview`<br>camera.capabilities, camera.view | Dùng số stream/chất lượng trong giới hạn hoặc đề xuất layout ít hơn<br>✓ Không mở bốn main stream vô điều kiện<br>✓ Focus vẫn dùng remote | Quyền xem từng camera |
| HC-033 · P0 | Quay camera sang trái một chút | PTZ hỗ trợ relative move; giới hạn tốc độ đã đặt | `camera` → `ptz_move`<br>camera.capabilities, camera.control | Gửi bước PTZ hữu hạn và dừng đúng protocol<br>✓ Không quay liên tục vì thiếu stop<br>✓ Không dịch chuyển camera không PTZ | Quyền camera control |
| HC-034 · P0 | Quay về vị trí cửa chính | Preset tồn tại trên camera đang chọn | `camera` → `recall_preset`<br>camera.control | Gọi đúng preset ID, báo kết quả<br>✓ Không áp preset ID của camera khác | Quyền camera control |
| HC-035 · P1 | Theo dõi người trong sân | Camera/worker chưa có capability tracking | `camera` → `request_tracking`<br>camera.capabilities | Báo chưa hỗ trợ, có thể chọn xem/PTZ thủ công<br>✓ Không giả đang theo dõi chỉ vì có person detection | Quyền control nếu tracking được triển khai |
| HC-036 · P0 | Dừng quay camera | Có PTZ session đang hoạt động | `camera` → `ptz_stop`<br>camera.control | Gửi stop đến camera của phiên điều khiển<br>✓ Không dừng player thay motor | Quyền camera control |
| HC-037 · P0 | Nói qua camera rằng tôi ra ngay | Camera có two-way audio; target rõ | `camera` → `speak`<br>camera.speak | Phát đúng nội dung qua camera đã chọn, báo kết quả truyền<br>✓ Không phát cùng câu qua mọi loa<br>✓ Không báo người ngoài đã nghe chắc chắn | User yêu cầu truyền lời rõ ràng; quyền camera speak |
| HC-038 · P0 | Bật mic để nghe camera | Camera có audio; user có quyền nghe | `camera` → `listen`<br>camera.capabilities, camera.listen | Mở audio stream và chỉ báo đang nghe<br>✓ Không giả có âm thanh nếu stream chỉ video<br>✓ Không ghi âm tự động | Quyền nghe camera |
| HC-039 · P0 | Tắt chế độ riêng tư camera | Camera hỗ trợ privacy; user có quyền | `camera` → `set_privacy`<br>camera.control, camera.state | Tắt privacy và xác minh trạng thái mới nếu đọc được<br>✓ Không chỉ đổi badge UI<br>✓ Không giả lệnh thành công khi camera timeout | Quyền thay đổi privacy |
| HC-040 · P1 | Chụp ảnh camera cửa lúc này | Snapshot capability có; đích lưu theo cấu hình | `camera` → `snapshot`<br>camera.snapshot | Ảnh có timestamp/camera ID; lưu nếu đích đã được chỉ định<br>✓ Không gửi ảnh ra dịch vụ khác<br>✓ Không giữ lâu hơn policy lưu trữ | Quyền snapshot và lưu ảnh nếu yêu cầu |

## Sự kiện cửa và an ninh — 10 ca

| ID / ưu tiên | Trigger / input | Điều kiện | Domain → intent / skill | Kết quả và tiêu chí | Quyền / ủy quyền |
| --- | --- | --- | --- | --- | --- |
| HC-041 · P0 | Chuông cửa reo khi đang xem phim | Doorbell event hợp lệ; TV đích đã đăng ký | `security` → `doorbell_notify`<br>event.validate, notification.route, camera.view | Hiện thông báo/camera nhỏ theo cấu hình, cho mở lớn<br>✓ Không chiếm màn hình ngoài policy<br>✓ Không tự mở khóa | Rule chuông cửa đã cấu hình |
| HC-042 · P1 | Cửa trước mở lúc nửa đêm | Contact sensor mới; khung giờ rule qua ngày | `security` → `door_open_alert`<br>home.state, automation.evaluate, notification.route | Cảnh báo có nguồn và thời điểm nếu condition đúng<br>✓ Không dùng giờ UTC sai ngày<br>✓ Không gọi đó là đột nhập chắc chắn | Rule cảnh báo đã bật |
| HC-043 · P0 | Có người trong vùng cấm khi away | Zone polygon và lịch rule đã chốt; có enter đã debounce | `security` → `restricted_zone_alert`<br>vision.presence, automation.evaluate, notification.route | Cảnh báo quan sát người ở vùng, giữ bằng chứng theo policy<br>✓ Không xác định người lạ chỉ từ person detection<br>✓ Không tự báo cơ quan bên ngoài | Rule và receiver được cấu hình |
| HC-044 · P1 | Có người đứng trước cửa lâu | Rule dwell time đã đặt, người vẫn quan sát liên tục | `security` → `loitering_alert`<br>vision.presence, automation.evaluate | Một cảnh báo sau dwell threshold, cooldown theo sự kiện<br>✓ Không lặp mỗi frame<br>✓ Không reset timer do jitter ngắn | Rule dwell-time đã bật |
| HC-045 · P0 | Mở khóa thất bại sau nhiều lần thử | Integration cung cấp attempt events; lock state chưa đổi | `security` → `lock_attempt_alert`<br>home.state, event.validate, notification.route | Cảnh báo attempt và trạng thái lock đọc được<br>✓ Không coi attempt thành unlocked<br>✓ Không retry unlock vô hạn | Quyền đọc sự kiện khóa; rule đã bật |
| HC-046 · P1 | Cửa mở lâu quá | Contact mở liên tục quá thời gian rule | `security` → `door_left_open`<br>automation.evaluate, home.state | Nhắc một lần theo cooldown; đóng cửa thì clear<br>✓ Không nhắc sau khi nhận close mới hơn | Rule đã cấu hình |
| HC-047 · P0 | User xác nhận đây là khách | Context gắn một event ID chuông cửa | `security` → `acknowledge_event`<br>event.acknowledge | Đánh dấu event đã xem/xử lý theo UI<br>✓ Không thêm danh tính vào nhận diện khuôn mặt<br>✓ Không tự mở khóa | Quyền xác nhận event |
| HC-048 · P1 | Xem lại sự kiện trước cửa lúc 8 giờ | NVR có recording và ngày chưa rõ | `camera` → `event_history`<br>camera.history, clarification.ask | Hỏi ngày nếu mơ hồ; tra clip từ recording source<br>✓ Không hứa có video nếu chỉ có event metadata | Quyền xem lịch sử và recording |
| HC-049 · P0 | Cảm biến khói báo động, camera không thấy gì | Smoke event từ sensor được cấu hình; camera không chứng minh an toàn | `security` → `sensor_alarm`<br>event.validate, notification.route | Hiển thị cảnh báo sensor rõ nguồn; action chỉ theo rule đã cho phép<br>✓ Không hạ cảnh báo vì camera trống<br>✓ Không tuyên bố đã kiểm soát sự cố | Rule cảnh báo và thiết bị nhận đã cấu hình |
| HC-050 · P0 | Bật riêng tư camera nhưng sensor cửa báo mở | Privacy camera đang bật; contact sensor được phép hoạt động | `security` → `privacy_respecting_alert`<br>home.state, notification.route | Thông báo sensor theo rule; giữ camera privacy<br>✓ Không tự bật lại camera để xác minh | Rule sensor không vượt quyền privacy |

## Âm thanh và phối hợp nhiều cảm biến — 10 ca

| ID / ưu tiên | Trigger / input | Điều kiện | Domain → intent / skill | Kết quả và tiêu chí | Quyền / ủy quyền |
| --- | --- | --- | --- | --- | --- |
| HC-051 · P0 | YAMNet nhận tiếng vỗ tay ở sofa | Có occupancy sofa mới; rule clap đã bật và debounce riêng | `audio` → `clap_trigger`<br>audio.classify, presence.fuse, automation.evaluate | Tạo clap event hợp lệ rồi gọi action theo rule<br>✓ Không coi top label là lệnh bật/tắt trực tiếp<br>✓ Không chạy nếu thiếu điều kiện rule | Rule và target được user cấu hình |
| HC-052 · P0 | Phim trên TV phát tiếng vỗ tay | Có playback audio và YAMNet clap; chưa có echo/source verification | `audio` → `ambiguous_sound`<br>audio.classify, event.arbitrate | Không kích hoạt action clap khi chưa phân biệt được nguồn theo policy<br>✓ Không tắt/mở TV lặp vì âm thanh tự phát | Cần rule phân biệt nguồn; không mặc định tin classifier |
| HC-053 · P0 | TV phát câu mở cửa | ASR nghe thấy câu nhưng không có phiên người dùng được xác nhận | `audio` → `untrusted_command`<br>speech.classify, event.arbitrate | Bỏ lệnh hành động hoặc yêu cầu tương tác người dùng<br>✓ Không mở cửa từ thoại phim<br>✓ Không coi transcript là authorization | Chỉ session input được phép mới gọi action |
| HC-054 · P1 | Nghe tiếng chuông báo nhưng chưa biết nguồn | YAMNet báo alarm class; không có smoke sensor xác nhận | `audio` → `possible_alarm`<br>audio.classify, notification.route | Thông báo có âm thanh giống cảnh báo, kèm độ tin cậy<br>✓ Không khẳng định có cháy<br>✓ Không tự coi đã an toàn khi điểm giảm | Rule thông báo audio đã bật |
| HC-055 · P1 | Micro phát hiện tiếng nói khi phòng tối | Không có wake interaction; có occupancy unknown | `audio` → `speech_observation`<br>audio.classify, presence.fuse | Cập nhật bằng chứng activity theo policy, không biến thành lệnh<br>✓ Không bật đèn vì mọi tiếng nói nếu chưa có rule | Quyền thu nhận audio theo cấu hình |
| HC-056 · P0 | Tiếng động mạnh rồi camera mất hình | Hai quan sát gần nhau nhưng không đủ kết luận sự cố | `security` → `correlate_observations`<br>event.correlate, notification.route | Thông báo mất camera kèm quan sát âm thanh nếu rule bật<br>✓ Không tuyên bố phá hoại<br>✓ Không bỏ qua mất tín hiệu | Rule tương quan đã cấu hình |
| HC-057 · P1 | Cảm biến cửa mở cùng person enter | Hai event có event/source timestamp khác nhau nhỏ | `presence` → `arrival_correlation`<br>event.correlate, automation.evaluate | Gộp thành một arrival theo cửa sổ thời gian rule<br>✓ Không chạy scene chào hai lần | Rule arrival đã bật |
| HC-058 · P0 | Camera thấy có người nhưng mic yên lặng | Vision còn mới, audio không phát speech | `presence` → `fuse_presence`<br>presence.fuse | Giữ occupied theo quan sát vision<br>✓ Không coi im lặng là vắng người | Nguồn presence được phép |
| HC-059 · P0 | YAMNet mất kết nối | TCP inference timeout; camera vẫn hoạt động | `audio` → `worker_unavailable`<br>audio.health | Đánh dấu audio unavailable, giữ chức năng khác<br>✓ Không trả is_alarm=false như kết quả inference<br>✓ Không crash pipeline camera | Quyền đọc worker health |
| HC-060 · P1 | Tiếng vỗ tay được phát hiện liên tiếp | Nhiều window audio chồng lấn của cùng một tiếng | `audio` → `deduplicate_sound`<br>event.deduplicate, automation.evaluate | Một trigger logic theo cửa sổ debounce/cooldown<br>✓ Không toggle thiết bị nhiều lần do overlap | Rule debounce đã cấu hình |

## Phối hợp TV, người dùng và thông báo — 10 ca

| ID / ưu tiên | Trigger / input | Điều kiện | Domain → intent / skill | Kết quả và tiêu chí | Quyền / ủy quyền |
| --- | --- | --- | --- | --- | --- |
| HC-061 · P0 | Sofa occupied trong lúc TV đang tắt | Có rule chào/mở TV đã bật; target TV rõ | `automation` → `presence_scene`<br>automation.evaluate, device.power | Thực hiện action chỉ khi đủ rule, báo kết quả thực tế<br>✓ Không tự bật TV chỉ vì vision có người nếu chưa bật rule | Rule đã ủy quyền trước |
| HC-062 · P0 | Phòng vắng nhưng phim vẫn phát | Room occupancy empty mới; policy giữ TV khi playback active | `automation` → `idle_room_check`<br>presence.fuse, player.state, automation.evaluate | Tôn trọng điều kiện playback và thời gian idle<br>✓ Không tắt TV ngay sau một leave event | Rule idle đã cấu hình |
| HC-063 · P0 | Không ai trong phòng và TV đã dừng đủ lâu | Healthy observation, idle threshold đã đủ | `automation` → `idle_shutdown`<br>automation.evaluate, device.power, home.control | Tắt đúng thiết bị theo rule, kết quả riêng từng action<br>✓ Không tắt thiết bị phòng khác<br>✓ Không bỏ qua điều kiện sensor stale | Rule idle đã bật |
| HC-064 · P1 | Thông báo chuông cửa khi TV standby | Client không hỗ trợ đánh thức hoặc không có quyền | `notification` → `route_alert`<br>device.capabilities, notification.route | Chọn receiver đã cấu hình khác hoặc ghi chưa giao được<br>✓ Không giả đã hiển thị TV<br>✓ Không đánh thức mọi TV | Rule và receiver đã cấu hình |
| HC-065 · P0 | Hai TV đang bật cùng lúc nhận sự kiện camera | Rule chỉ định TV phòng khách | `notification` → `targeted_delivery`<br>device.resolve, notification.route | Gửi đúng TV phòng khách, có delivery state<br>✓ Không broadcast theo mặc định | Quyền gửi tới receiver đích |
| HC-066 · P1 | Thông báo không khẩn cấp trong giờ ngủ | Quiet-hours policy đang áp dụng | `notification` → `quiet_delivery`<br>notification.policy, notification.route | Hiện yên lặng/hoãn theo policy đã lưu<br>✓ Không phát loa hoặc TTS vì mặc định | Policy thông báo user đã cấu hình |
| HC-067 · P0 | User dismiss cảnh báo trên TV | Notification ID rõ, nguồn sensor vẫn đang active | `notification` → `dismiss`<br>notification.acknowledge | Ẩn/đánh dấu đã đọc theo scope UI<br>✓ Không xóa sensor alarm hoặc tắt bảo vệ tại nguồn | Quyền acknowledge notification |
| HC-068 · P0 | User đang kéo rèm thủ công khi scene bắt đầu | Hai action cạnh tranh cùng cover | `automation` → `arbitrate_control`<br>event.arbitrate, home.state | Áp policy ưu tiên thao tác tay và ghi kết quả bị bỏ/hoãn<br>✓ Không gửi lệnh đảo chiều liên tục<br>✓ Không báo scene hoàn tất toàn bộ | Policy arbitration đã cấu hình |
| HC-069 · P1 | Khách yêu cầu xem camera phòng ngủ | Profile guest không có quyền camera đó | `camera` → `authorize_view`<br>authorization.check | Từ chối quyền, không trả snapshot/stream URL<br>✓ Không rò ảnh qua thumbnail hoặc cache<br>✓ Không mượn quyền server admin | Quyền theo profile hiện tại |
| HC-070 · P1 | Tôi muốn biết vì sao đèn vừa bật | Có trace trigger, condition, tool result | `automation` → `explain_action`<br>automation.trace | Nêu rule, tín hiệu và kết quả thực tế bằng ngôn ngữ dễ hiểu<br>✓ Không bịa nguyên nhân khi thiếu trace<br>✓ Không lộ credentials trong trace | Quyền đọc trace liên quan |

## Lỗi, cạnh tranh và phục hồi — 10 ca

| ID / ưu tiên | Trigger / input | Điều kiện | Domain → intent / skill | Kết quả và tiêu chí | Quyền / ủy quyền |
| --- | --- | --- | --- | --- | --- |
| HC-071 · P0 | Webhook camera gửi lặp cùng event | Producer có event ID; consumer retry sau timeout | `event` → `idempotent_delivery`<br>event.validate, event.deduplicate | Một logical event và action; retry có delivery status<br>✓ Không bật scene lại mỗi lần nhận retry | Event producer được xác thực |
| HC-072 · P0 | Leave cũ đến sau enter mới | Event có source ID, timestamp và thứ tự khi hỗ trợ | `presence` → `out_of_order_event`<br>event.order, presence.fuse | Loại/đánh dấu event cũ theo ordering policy<br>✓ Không chuyển phòng trống vì leave trễ | Chỉ nguồn được phép cập nhật state |
| HC-073 · P0 | Đồng hồ camera lệch nhiều phút | Timestamp producer không đáng tin; có received_at | `event` → `clock_skew`<br>event.validate, event.order | Giữ source time và receive time, đánh dấu độ mới chưa chắc<br>✓ Không dùng timestamp lệch làm dữ liệu live chắc chắn | Nguồn được cấu hình; kiểm tra freshness |
| HC-074 · P0 | HA mất kết nối khi gọi tắt đèn | Không biết action đã được HA nhận chưa | `home` → `uncertain_action_result`<br>home.control, home.state | Báo trạng thái chưa xác nhận, read-after-reconnect khi khả thi<br>✓ Không báo đã tắt<br>✓ Không retry action nhạy cảm mù quáng | Quyền control/read theo target |
| HC-075 · P0 | Một scene thành công một phần | Hai đèn thành công, rèm thất bại | `home` → `partial_execution`<br>home.scene, home.state | Báo kết quả từng thành phần và tùy chọn thử lại phần lỗi<br>✓ Không báo scene thành công toàn bộ<br>✓ Không rollback gây tác dụng phụ ngoài policy | Quyền scene/action đã cấu hình |
| HC-076 · P0 | Server khởi động lại khi đang đếm idle | Không có dữ liệu tươi sau restart; timer có hoặc không persist | `automation` → `recover_state`<br>automation.recover, presence.fuse | Khôi phục có kiểm tra freshness hoặc bắt đầu quan sát lại<br>✓ Không coi uptime mới bằng thời gian phòng vắng<br>✓ Không tự tắt ngay | Rule đang bật trước restart |
| HC-077 · P0 | Redis chứa occupancy cũ nhưng camera đã offline | Cached state quá freshness budget | `presence` → `stale_cache`<br>cache.lookup, presence.fuse | Hiển thị stale/unknown; không dùng để tắt thiết bị<br>✓ TTL cache không thay thế độ mới observation<br>✓ Không coi giá trị cuối là live | Quyền đọc scope presence |
| HC-078 · P1 | Camera đổi mật khẩu hoặc token hết hạn | API trả auth error, stream cũng không truy cập được | `camera` → `reauth_required`<br>camera.health, account.status | Báo cần cập nhật credentials đúng camera<br>✓ Không log mật khẩu<br>✓ Không thử lại liên tục gây lockout | Chỉ admin cấu hình được cập nhật credentials |
| HC-079 · P0 | Nhiều trigger đến khi action trước chưa xong | Mode rule đã chọn single/restart/queued/parallel với giới hạn | `automation` → `execution_mode`<br>automation.evaluate, event.arbitrate | Thực hiện đúng mode và giới hạn queue/concurrency của rule<br>✓ Không chạy song song mặc định<br>✓ Không kéo queue vô hạn | Rule/mode đã được lưu trước |
| HC-080 · P0 | User thu hồi quyền camera khi đang xem | Phiên xem đã cấp trước đó; quyền mới không cho phép | `camera` → `revoke_access`<br>authorization.check, camera.session_revoke | Dừng phiên hoặc vô hiệu resource token theo chính sách có hiệu lực<br>✓ Không cho tiếp tục xem vô hạn bằng URL cũ<br>✓ Không phục vụ ảnh cache vượt quyền | Thay đổi quyền từ quản trị viên được phép |

## Phần triển khai còn thiếu

Corpus này chưa được nối vào runner preview. Cần schema input/event có context và quyền,
source health/freshness, deduplication/ordering, trace, capability và policy arbitration.
Cần kiểm chứng actual tool results trước khi báo thành công; một số rule hiện có mới gửi
request và chưa đủ read-back/result aggregation để đáp ứng các assertions ở đây.
Không tạo thêm runtime hoặc thư mục implementation chỉ để phản chiếu từng nhóm test.
