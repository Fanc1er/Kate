// Package main 是 CInsight Master 进程入口（根目录 go run . 即启动 Master）。
package main

import "github.com/Fanc1er/Kate/backend/internal/master/app"

func main() {
	app.Run()
}
