package database

import "strings"

// 保存每个命令和命令对应的command结构体
var cmdTable = make(map[string]*command)

// 每个命令对应一个command
type command struct {
	// 在 ExecFunc 执行前执行，负责分析命令行读写了哪些 key 便于进行加锁
	prepare PreFunc
	// 仅在事务中被使用，负责准备 undo logs 以备事务执行过程中遇到错误需要回滚
	undo UndoFunc
	// 实际执行命令的函数
	executor ExecFunc
	// 参数数量
	arity int
}

// RegisterCommand 在map中注册命令
func RegisterCommand(name string, executor ExecFunc, prepare PreFunc, rollback UndoFunc, arity int) {
	name = strings.ToLower(name)
	cmdTable[name] = &command{
		executor: executor,
		prepare:  prepare,
		undo:     rollback,
		arity:    arity,
	}
}
