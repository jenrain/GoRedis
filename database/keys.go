package database

import (
	"GoRedis/datastruct/dict"
	List "GoRedis/datastruct/list"
	"GoRedis/datastruct/set"
	"GoRedis/datastruct/sortedset"
	"GoRedis/interface/resp"
	"GoRedis/lib/utils"
	"GoRedis/lib/wildcard"
	"GoRedis/resp/reply"
)

// DEL K1 K2 K3
func execDel(db *DB, args [][]byte) resp.Reply {
	keys := make([]string, len(args))
	for i, v := range args {
		keys[i] = string(v)
	}
	deleted := db.Removes(keys...)
	if deleted > 0 {
		db.addAof(utils.ToCmdLine2("del", args...))
	}
	return reply.MakeIntReply(int64(deleted))
}

// EXISTS k1 k2 k3 ...
func execExists(db *DB, args [][]byte) resp.Reply {
	result := int64(0)
	for _, arg := range args {
		key := string(arg)
		_, exists := db.GetEntity(key)
		if exists {
			result++
		}
	}
	return reply.MakeIntReply(result)
}

// FLUSHDB
func execFlushDB(db *DB, args [][]byte) resp.Reply {
	db.Flush()
	db.addAof(utils.ToCmdLine2("flushdb", args...))
	return reply.MakeOkReply()
}

// TYPE k1
func execType(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	entity, exists := db.GetEntity(key)
	if !exists {
		return reply.MakeStatusReply("none")
	}
	switch entity.Data.(type) {
	case []byte:
		return reply.MakeStatusReply("string")
	case List.List:
		return reply.MakeStatusReply("list")
	case dict.Dict:
		return reply.MakeStatusReply("hash")
	case *set.Set:
		return reply.MakeStatusReply("set")
	case *sortedset.SortedSet:
		return reply.MakeStatusReply("zset")

	}
	return &reply.UnknownErrRepl{}
}

// RENAME k1 k2
func execRename(db *DB, args [][]byte) resp.Reply {
	src := string(args[0])
	dest := string(args[1])
	entity, exists := db.GetEntity(src)
	if !exists {
		return reply.MakeErrReply("no such key")
	}
	db.PutEntity(dest, entity)
	db.Remove(src)
	db.addAof(utils.ToCmdLine2("rename", args...))
	return reply.MakeOkReply()
}

// RENAMENX
func execRenamenx(db *DB, args [][]byte) resp.Reply {
	src := string(args[0])
	dest := string(args[1])

	_, ok := db.GetEntity(dest)
	if ok {
		return reply.MakeIntReply(0)
	}

	entity, exists := db.GetEntity(src)
	if !exists {
		return reply.MakeErrReply("no such key")
	}
	db.PutEntity(dest, entity)
	db.Remove(src)
	db.addAof(utils.ToCmdLine2("renamenx", args...))
	return reply.MakeOkReply()
}

// KEYS *
func execKeys(db *DB, args [][]byte) resp.Reply {
	pattern := wildcard.CompilePattern(string(args[0]))
	result := make([][]byte, 0)
	db.data.ForEach(func(key string, val interface{}) bool {
		if pattern.IsMatch(key) {
			result = append(result, []byte(key))
		}
		return true
	})
	return reply.MakeMultiBulkReply(result)
}

func undoDel(db *DB, args [][]byte) []CmdLine {
	keys := make([]string, len(args))
	for i, v := range args {
		keys[i] = string(v)
	}
	return rollbackGivenKeys(db, keys...)
}

func undoRename(db *DB, args [][]byte) []CmdLine {
	src := string(args[0])
	dest := string(args[1])
	return rollbackGivenKeys(db, src, dest)
}

func prepareRename(args [][]byte) ([]string, []string) {
	src := string(args[0])
	dest := string(args[1])
	return []string{dest}, []string{src}
}

func init() {
	RegisterCommand("DEL", execDel, writeAllKeys, undoDel, -2)
	RegisterCommand("EXISTS", execExists, readAllKeys, nil, -2)
	RegisterCommand("type", execType, readFirstKey, nil, 2)
	RegisterCommand("RENAME", execRename, prepareRename, undoRename, 3)
	RegisterCommand("RENAMENX", execRenamenx, prepareRename, undoRename, 3)
	RegisterCommand("KEYS", execKeys, noPrepare, nil, 2)
}
