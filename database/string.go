package database

import (
	"GoRedis/interface/database"
	"GoRedis/interface/resp"
	"GoRedis/lib/utils"
	"GoRedis/resp/reply"
)

// GET k1
func execGet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	entity, exists := db.GetEntity(key)
	if !exists {
		return reply.MakeNullBulkReply()
	}
	bytes := entity.Data.([]byte)
	return reply.MakeBulkReply(bytes)
}

// SET k v
func execSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]
	entity := &database.DataEntity{Data: value}
	db.PutEntity(key, entity)
	db.addAof(utils.ToCmdLine2("set", args...))
	return reply.MakeOkReply()
}

// SETNX
func execSetnx(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]
	entity := &database.DataEntity{Data: value}
	result := db.PutIfAbsent(key, entity)
	db.addAof(utils.ToCmdLine2("setnx", args...))
	return reply.MakeIntReply(int64(result))
}

// GETSET
func execGetSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]
	entity, exists := db.GetEntity(key)
	db.PutEntity(key, &database.DataEntity{Data: value})
	if !exists {
		return reply.MakeNullBulkReply()
	}
	db.addAof(utils.ToCmdLine2("getset", args...))
	return reply.MakeBulkReply(entity.Data.([]byte))
}

// STRLEN
func execStrLen(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	entity, exists := db.GetEntity(key)
	if !exists {
		return reply.MakeNullBulkReply()
	}
	bytes := entity.Data.([]byte)
	return reply.MakeIntReply(int64(len(bytes)))
}

func init() {
	RegisterCommand("Get", execGet, readFirstKey, nil, 2)
	RegisterCommand("Set", execSet, writeFirstKey, rollbackFirstKey, 3)
	RegisterCommand("SetNx", execSetnx, writeFirstKey, rollbackFirstKey, 3)
	RegisterCommand("GetSet", execGetSet, writeFirstKey, rollbackFirstKey, 3)
	RegisterCommand("StrLen", execStrLen, readFirstKey, nil, 2)
}
