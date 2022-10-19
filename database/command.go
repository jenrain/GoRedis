package database

import "strings"

// 保存每个命令和命令对应的command结构体
var cmdTable = make(map[string]*command)

// 每个命令对应一个command
type command struct {
	// 每个指令对应一个方法（执行模式）
	exector ExecFunc
	// 参数数量
	arity int
}

// RegisterCommand 在map中注册命令
func RegisterCommand(name string, exector ExecFunc, arity int) {
	name = strings.ToLower(name)
	cmdTable[name] = &command{
		exector: exector,
		arity:   arity,
	}
}
