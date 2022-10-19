package main

import (
	"GoRedis/config"
	"GoRedis/lib/logger"
	"GoRedis/resp/handler"
	"GoRedis/tcp"
	"fmt"
	"os"
)

// 设置配置文件的名字
const configFile string = "redis.conf"

// 默认配置
var defaultProperties = &config.ServerProperties{
	Bind: "0.0.0.0",
	Port: 63791,
}

// 判断文件是否存在
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	return err == nil && !info.IsDir()
}

// 1. 初始化日志
// 2. 导入配置文件
func main() {
	// 初始化日志
	logger.Setup(&logger.Settings{
		Path:       "logs",
		Name:       "GoRedis",
		Ext:        "log",
		TimeFormat: "2006-01-02",
	})

	// 判断配置文件是否存在
	if fileExists(configFile) {
		config.SetupConfig(configFile)
	} else {
		config.Properties = defaultProperties
	}

	err := tcp.ListenAndServeWithSignal(
		&tcp.Config{
			Address: fmt.Sprintf("%s:%d", config.Properties.Bind, config.Properties.Port),
		},
		// 调用resp层的handler
		handler.MakeHandler())
	if err != nil {
		logger.Error(err)
	}
}
