package contenidos

import "embed"

//go:embed *.json
var Defaults embed.FS
