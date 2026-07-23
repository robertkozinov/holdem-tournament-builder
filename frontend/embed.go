package frontend

import "embed"

// Files contains the frontend assets served by the application.
//
//go:embed index.html styles.css js/*.js vendor/*.css
var Files embed.FS
