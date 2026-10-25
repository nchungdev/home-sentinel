package web

import (
	"bytes"
	"embed"
	"io/fs"
	"sort"
	"sync"
)

//go:embed styles/*.css views/*.html scripts/*.js
var assetsFS embed.FS

var (
	cachedHTML []byte
	buildOnce  sync.Once
)

func appendEmbeddedDir(buf *bytes.Buffer, dir string) {
	entries, err := fs.ReadDir(assetsFS, dir)
	if err != nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := assetsFS.ReadFile(dir + "/" + entry.Name())
		if err == nil {
			buf.Write(data)
		}
	}
}

func GetIndexHTML() []byte {
	buildOnce.Do(func() {
		var buf bytes.Buffer
		buf.WriteString(`<!doctype html>
<html lang="vi">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="dark">
  <title>Home Sentinel</title>
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Material+Symbols+Rounded:opsz,wght,FILL,GRAD@20..48,400,0,0" />
  <style>
`)
		appendEmbeddedDir(&buf, "styles")
		buf.WriteString("  </style>\n</head>\n<body>\n")
		appendEmbeddedDir(&buf, "views")
		buf.WriteString("  <script>\n")
		appendEmbeddedDir(&buf, "scripts")
		buf.WriteString("  </script>\n</body>\n</html>\n")
		cachedHTML = buf.Bytes()
	})
	return cachedHTML
}
