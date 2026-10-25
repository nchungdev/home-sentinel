# Home Sentinel Architecture & Backend Tasks

Lộ trình thiết kế, phân tầng và chuyển đổi kiến trúc 6 tầng trên Home Sentinel (Go Backend).

## Danh sách Task chi tiết

| Mã Task | Tên Task | Trạng thái |
|---|---|---|
| [M0-T0.1](M0-T0.1-verify-runtime-and-baseline.md) | Kiểm tra runtime NAS, đối chiếu Go/Python và baseline | Hoàn thành |
| [M1-T1.3](M1-T1.3-contract-fixtures-and-validation.md) | Bộ JSON fixtures và test kiểm tra tính tương thích Go | Hoàn thành |
| [M2-T2.1](M2-T2.1-transport-layer-isolation.md) | Tách tầng Transport (Auth, Deadline, Connection pool) | Hoàn thành |
| [M2-T2.2](M2-T2.2-orchestration-pipeline.md) | Xây dựng PipelineOrchestrator 6 tầng & Legacy Adapter | Hoàn thành |
| [M3-T3.1](M3-T3.1-understanding-phonetic-rules.md) | Tách Understanding (Chuẩn hóa tiếng Việt, Say Hi -> C2) | Hoàn thành |
| [M3-T3.2](M3-T3.2-resolution-engine.md) | Tách Resolution (Đối chiếu ID thật HA/Plex, sinh ActionPlan) | Hoàn thành |
| [M3-T3.3](M3-T3.3-proactive-suggestion-engine.md) | Xây dựng SuggestionEngine chọn gợi ý bằng dữ liệu thật | Hoàn thành |
| [M4-T4.1](M4-T4.1-v2-ndjson-endpoint.md) | Mở endpoint `/v2/ask` stream NDJSON có cấu trúc | Hoàn thành |
| [M5-T5.1](M5-T5.1-ha-adapter-migration.md) | Di chuyển lệnh Home Assistant sang Adapter chuẩn hóa | Hoàn thành |
| [M5-T5.2](M5-T5.2-camera-adapter-migration.md) | Di chuyển lệnh Camera Tapo sang Adapter chuẩn hóa | Hoàn thành |
| [M5-T5.3](M5-T5.3-media-event-sync.md) | Cơ chế đồng bộ sự kiện Radarr/Sonarr tải xong về TV | Hoàn thành |
| [M6-T6.1](M6-T6.1-observability-request-id.md) | Gắn request_id đo lường độ trễ từng tầng | Hoàn thành |
| [M6-T6.2](M6-T6.2-qwen-admission-control.md) | Giới hạn cgroup và điều tiết tải Local Qwen trên NAS | Hoàn thành |
| [M6-T6.3](M6-T6.3-e2e-verification-and-cutover.md) | Kiểm thử 100 test case end-to-end và chuyển mặc định | Hoàn thành |
