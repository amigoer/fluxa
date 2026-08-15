// Package web embeds the built frontend into the server binary, so the
// deployment stays a single artifact (DESIGN.md section 5: "前端构建产物
// 打包嵌入后端服务"). dist/ is produced by `npm run build` in frontend/
// and is not checked into version control.
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
