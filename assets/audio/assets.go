// Package audioassets embeds the original Kenney CC0 sound candidates.
package audioassets

import "embed"

// Files preserves the original filenames; Vorbis TITLE tags are not identifiers.
//
//go:embed *.ogg
var Files embed.FS
