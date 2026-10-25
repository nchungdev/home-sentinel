package web

import (
	_ "embed"
	"os"
)

//go:embed index.html
var defaultIndexHTML []byte

func GetIndexHTML() []byte {
	for _, p := range []string{"web/index.html", "../../web/index.html"} {
		if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
			return data
		}
	}
	return defaultIndexHTML
}
