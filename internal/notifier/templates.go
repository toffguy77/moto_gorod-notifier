package notifier

import (
	"embed"
)

//go:embed templates/*.tmpl
var templateFS embed.FS
