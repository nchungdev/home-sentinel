package mediasearch

import (
	"strings"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

func contentTypes(sources []contracts.MediaSourceResults) []contracts.MediaContentType {
	contentTypes := []contracts.MediaContentType{}
	typeIndexes := make(map[string]int)

	for _, source := range sources {
		itemsByType := make(map[string][]contracts.MediaItem)
		typeOrder := []string{}
		for _, item := range source.Items {
			contentType := mediaContentType(item)
			if _, found := itemsByType[contentType]; !found {
				typeOrder = append(typeOrder, contentType)
			}
			itemsByType[contentType] = append(itemsByType[contentType], item)
		}

		for _, contentType := range typeOrder {
			typeIndex, found := typeIndexes[contentType]
			if !found {
				typeIndex = len(contentTypes)
				typeIndexes[contentType] = typeIndex
				contentTypes = append(contentTypes, contracts.MediaContentType{
					Type: contentType, Sections: []contracts.MediaContentSection{},
				})
			}
			contentTypes[typeIndex].Sections = append(contentTypes[typeIndex].Sections,
				contracts.MediaContentSection{
					SourceID: source.SourceID, Status: source.Status, Searched: source.Searched,
					Cache: source.Cache, Items: itemsByType[contentType], ErrorCode: source.ErrorCode,
				})
		}
	}

	return contentTypes
}

func mediaContentType(item contracts.MediaItem) string {
	switch strings.ToLower(strings.TrimSpace(item.Kind)) {
	case "movie":
		return "movie"
	case "show", "series", "season", "episode":
		return "series"
	case "video", "clip", "clips":
		return "videos"
	case "wiki", "web", "web_link", "information", "article":
		return "other"
	default:
		return "other"
	}
}
