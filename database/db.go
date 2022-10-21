package database

import (
	"GoRedis/datastruct/dict"
	"GoRedis/interface/database"
	"GoRedis/interface/resp"
	"GoRedis/resp/reply"
	"strings"
)

type DB struct {
	index int
	data  dict.Dict
	// aof持久化
	addAof func(line CmdLine)
	// 事务相关
	// 版本map
	versionMap dict.Dict
}

// PreFunc 在 ExecFunc 执行前执行，负责分析命令行读写了哪些 key
type PreFunc func(args [][]byte) ([]string, []string)

// ExecFunc 实际执行命令的函数
type ExecFunc func(db *DB, args [][]byte) resp.Reply

// UndoFunc 仅在事务中被使用，负责准备 undo logs 以备事务执行过程中遇到错误需要回滚
type UndoFunc func(db *DB, args [][]byte) []CmdLine

type CmdLine = [][]byte

func makeDB() *DB {
	db := &DB{
		data:       dict.MakeSyncDict(),
		addAof:     func(line CmdLine) {},
		versionMap: dict.MakeSyncDict(),
	}
	return db
}

func (db *DB) Exec(c resp.Connection, cmdLine CmdLine) resp.Reply {
	cmdName := strings.ToLower(string(cmdLine[0]))
	// 开始事务
	if cmdName == "multi" {
		if len(cmdLine) != 1 {
			return reply.MakeArgNumErrReply(cmdName)
		}
		return StartMulti(c)
		// 取消事务
	} else if cmdName == "discard" {
		if len(cmdLine) != 1 {
			return reply.MakeArgNumErrReply(cmdName)
		}
		return DiscardMulti(c)
		// 执行事务
	} else if cmdName == "exec" {
		if len(cmdLine) != 1 {
			return reply.MakeArgNumErrReply(cmdName)
		}
		return execMulti(db, c)
	} else if cmdName == "watch" {
		if !validateArity(-2, cmdLine) {
			return reply.MakeArgNumErrReply(cmdName)
		}
		return Watch(db, c, cmdLine[1:])
	}
	// 如果开启了事务，就将命令放入事务队列中
	if c != nil && c.InMultiState() {
		return EnqueueCmd(c, cmdLine)
	}

	return db.NormalExec(cmdLine)
}

func (db *DB) NormalExec(cmdLine CmdLine) resp.Reply {
	cmdName := strings.ToLower(string(cmdLine[0]))
	cmd, ok := cmdTable[cmdName]
	if !ok {
		return reply.MakeErrReply("ERR unknown command '" + cmdName + "'")
	}
	if !validateArity(cmd.arity, cmdLine) {
		return reply.MakeArgNumErrReply(cmdName)
	}
	prepare := cmd.prepare
	write, _ := prepare(cmdLine[1:])
	db.addVersion(write...)
	fun := cmd.executor
	return fun(db, cmdLine[1:])
}

// 校验参数个数
func validateArity(arity int, cmdArgs [][]byte) bool {
	// SET k v -> arity = 3
	// EXIST k1 k2 arity = -2
	argNum := len(cmdArgs)
	if arity >= 0 {
		return argNum == arity
	}
	return argNum >= -arity
}

func (db *DB) GetEntity(key string) (*database.DataEntity, bool) {
	raw, ok := db.data.Get(key)
	if !ok {
		return nil, false
	}
	entity, _ := raw.(*database.DataEntity)
	return entity, true
}

func (db *DB) PutEntity(key string, entity *database.DataEntity) int {
	return db.data.Put(key, entity)
}

func (db *DB) PutIfEntity(key string, entity *database.DataEntity) int {
	return db.data.PutIfExist(key, entity)
}

func (db *DB) PutIfAbsent(key string, entity *database.DataEntity) int {
	return db.data.PutIfAbsent(key, entity)
}

func (db *DB) Remove(key string) {
	db.data.Remove(key)
}

func (db *DB) Removes(keys ...string) (deleted int) {
	deleted = 0
	for _, key := range keys {
		_, exists := db.data.Get(key)
		if exists {
			db.Remove(key)
			deleted++
		}
	}
	return deleted
}

func (db *DB) Flush() {
	db.data.Clear()
}

/*
 * 事务相关
 */
// 更新key的版本
func (db *DB) addVersion(keys ...string) {
	for _, key := range keys {
		versionCode := db.GetVersion(key)
		db.versionMap.Put(key, versionCode+1)
	}
}

// GetVersion 返回给定key的版本
func (db *DB) GetVersion(key string) uint32 {
	entity, ok := db.versionMap.Get(key)
	if !ok {
		return 0
	}
	return entity.(uint32)
}
