package frontend

import "embed"

// Files contains the frontend assets served by the application.
//
//go:embed index.html styles.css js/*.js js/*.mjs vendor/*.css
var Files embed.FS
