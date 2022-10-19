package database

/*-- Redis的业务核心 --*/

import "GoRedis/interface/resp"

// CmdLine [][]byte的别名，代表命令行
type CmdLine = [][]byte

// Database 是Redis的业务核心
type Database interface {
	Exec(client resp.Connection, args [][]byte) resp.Reply
	AfterClientClose(c resp.Connection)
	Close()
}

// DataEntity 可以绑定Redis的任何数据结构
type DataEntity struct {
	Data interface{}
}
