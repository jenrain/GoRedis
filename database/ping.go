package database

import (
	"GoRedis/interface/resp"
	"GoRedis/resp/reply"
)

func Ping(db *DB, args [][]byte) resp.Reply {
	return reply.MakePongReply()
}

// 注册ping命令
func init() {
	RegisterCommand("ping", Ping, noPrepare, nil, 1)
}
