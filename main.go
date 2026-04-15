package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gameperf/internal/api"
	"gameperf/internal/db"
)

func main() {
	port := flag.String("port", "9090", "服务端口")
	dataDir := flag.String("data", "./data", "数据存储目录")
	flag.Parse()

	// 确保数据目录存在
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		fmt.Printf("创建数据目录失败: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(*dataDir, "gameperf.db")
	database, err := db.New(dbPath)
	if err != nil {
		fmt.Printf("初始化数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	server := api.NewServer(database)
	addr := fmt.Sprintf(":%s", *port)
	if err := server.Run(addr); err != nil {
		fmt.Printf("服务启动失败: %v\n", err)
		os.Exit(1)
	}
}
