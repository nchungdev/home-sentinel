package skills

import (
	"context"
	"strings"
)

func (s MovieMetadata) description(ctx context.Context, identifier, kind string) string {
	var detail struct {
		Overview  string `json:"overview"`
		CreatedBy []struct {
			Name string `json:"name"`
		} `json:"created_by"`
		Credits struct {
			Crew []struct {
				Name string `json:"name"`
				Job  string `json:"job"`
			} `json:"crew"`
		} `json:"credits"`
	}
	if s.read(ctx, "/"+identifier+"?language=vi-VN&append_to_response=credits", &detail) != "" {
		return ""
	}
	people, label := []string{}, "Đạo diễn: "
	if kind == "movie" {
		for _, member := range detail.Credits.Crew {
			if member.Job == "Director" {
				people = append(people, member.Name)
			}
		}
	} else {
		label = "Sáng tạo: "
		for _, creator := range detail.CreatedBy {
			people = append(people, creator.Name)
		}
	}
	if len(people) == 0 {
		return detail.Overview
	}

	return label + strings.Join(people, ", ") + "\n" + detail.Overview
}
