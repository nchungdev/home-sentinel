package contracts

import "fmt"

func MediaPresentation(report *MediaSearchResponse, complete bool) *PresentationResponse {
	count := 0
	cards := []map[string]any{}
	for _, source := range report.Sources {
		for _, item := range source.Items {
			count++
			actions := []map[string]any{}
			for _, action := range item.Actions {
				if !action.Enabled {
					continue
				}
				entry := map[string]any{"id": action.Type, "label": action.Label,
					"type": "server", "action_id": "media." + action.Type,
					"parameters": map[string]any{"source_id": action.SourceID, "item_id": action.ItemID,
						"url": action.URL, "season": action.Season}}
				if action.Type == "open_url" {
					entry["type"], entry["action_type"] = "client", "media.open"
					entry["parameters"] = map[string]any{"uri": action.URL}
				}
				actions = append(actions, entry)
			}
			cards = append(cards, map[string]any{"id": source.SourceID + ":" + item.ID, "title": item.Title,
				"year": item.Year, "media_type": item.Kind, "status": item.Availability,
				"source_id": source.SourceID, "link": item.URL, "actions": actions})
		}
	}
	speech := fmt.Sprintf("Đã tìm thấy %d kết quả. Các nguồn khác đang tiếp tục tìm…", count)
	if complete {
		speech = fmt.Sprintf("Tìm được %d kết quả từ các nguồn khả dụng.", count)
	}
	return &PresentationResponse{SpeechText: speech, Cards: cards, MediaSearch: report, Mood: "idle",
		Handler: &Handler{Route: "rule", Intent: "media.search"}}
}
