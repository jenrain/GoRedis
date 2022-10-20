package database

import (
	Dict "GoRedis/datastruct/dict"
	"GoRedis/interface/database"
	"GoRedis/interface/resp"
	"GoRedis/lib/utils"
	"GoRedis/resp/reply"
)

// 获取数据表中的字典结构
func (db *DB) getAsDict(key string) (Dict.Dict, reply.ErrorReply) {
	entity, exists := db.GetEntity(key)
	if !exists {
		return nil, nil
	}
	dict, ok := entity.Data.(Dict.Dict)
	if !ok {
		return nil, &reply.WrongTypeErrReply{}
	}
	return dict, nil
}

// 返回dict或者初始化
// inited标记有没有初始化dict
func (db *DB) getOrInitDict(key string) (dict Dict.Dict, inited bool, errReply reply.ErrorReply) {
	dict, errReply = db.getAsDict(key)
	if errReply != nil {
		return nil, false, errReply
	}
	inited = false
	// 需要创建
	if dict == nil {
		dict = Dict.MakeSimpleDict()
		db.PutEntity(key, &database.DataEntity{
			Data: dict,
		})
		inited = true
	}
	return dict, inited, nil
}

// 插入一个键值对
func execHSet(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])
	field := string(args[1])
	value := args[2]

	// 获取或者初始化dict
	dict, _, errReply := db.getOrInitDict(key)
	if errReply != nil {
		return errReply
	}

	result := dict.Put(field, value)
	db.addAof(utils.ToCmdLine2("hset", args...))
	return reply.MakeIntReply(int64(result))
}

// 当key不存在时插入一个新的键值对
func execHSetNX(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])
	field := string(args[1])
	value := args[2]

	dict, _, errReply := db.getOrInitDict(key)
	if errReply != nil {
		return errReply
	}

	// field不存在才插入
	result := dict.PutIfAbsent(field, value)
	if result > 0 {
		db.addAof(utils.ToCmdLine2("hsetnx", args...))
	}
	return reply.MakeIntReply(int64(result))
}

// 获取key对应的value
func execHGet(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])
	field := string(args[1])

	dict, errReply := db.getAsDict(key)
	if errReply != nil {
		return errReply
	}

	// 数据库中没有该dict
	if dict == nil {
		return &reply.NullBulkReply{}
	}

	raw, exists := dict.Get(field)
	// dict没有该字段
	if !exists {
		return &reply.NullBulkReply{}
	}
	value, _ := raw.([]byte)
	return reply.MakeBulkReply(value)
}

// 判断key是否存在
// 存在回复1，不存在回复0
func execHExists(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])
	field := string(args[1])

	// 数据库中没有该dict
	dict, errReply := db.getAsDict(key)
	if errReply != nil {
		return errReply
	}
	// 不存在
	if dict == nil {
		return reply.MakeIntReply(int64(0))
	}
	_, exists := dict.Get(field)
	// 存在
	if exists {
		return reply.MakeIntReply(int64(1))
	}
	return reply.MakeIntReply(int64(0))
}

// 删除一个或多个键值对
// 回复删除的键值对个数
func execHDel(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])
	fields := make([]string, len(args)-1)
	fieldsArgs := args[1:]
	for i, v := range fieldsArgs {
		fields[i] = string(v)
	}

	// 获取数据库dict结构
	dict, errReply := db.getAsDict(key)
	if errReply != nil {
		return errReply
	}

	// 数据库中无dict结构
	if dict == nil {
		return reply.MakeIntReply(int64(0))
	}

	deleted := 0
	for _, field := range fields {
		// 删除了返回1，不删除返回0
		result := dict.Remove(field)
		deleted += result
	}
	// dict里面已经没有键值对，清除这个dict
	if dict.Len() == 0 {
		db.Remove(key)
	}
	if deleted > 0 {
		db.addAof(utils.ToCmdLine2("hdel", args...))
	}
	return reply.MakeIntReply(int64(deleted))
}

// 获取哈希表中的键值对个数
func execHLen(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])

	dict, errReply := db.getAsDict(key)
	if errReply != nil {
		return errReply
	}
	// 数据库中无dict结构
	if dict == nil {
		return reply.MakeIntReply(int64(0))
	}
	return reply.MakeIntReply(int64(dict.Len()))
}

// HMSET key field1 value1 [field2 value2 ]
func execHMSet(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	if len(args)%2 != 1 {
		// 语法错误
		return reply.MakeSyntaxErrReply()
	}
	key := string(args[0])
	size := (len(args) - 1) / 2
	fields := make([]string, size)
	values := make([][]byte, size)
	for i := 0; i < size; i++ {
		fields[i] = string(args[2*i+1])
		values[i] = args[2*i+2]
	}

	// 获取或者初始化dict结构
	dict, _, errReply := db.getOrInitDict(key)
	if errReply != nil {
		return errReply
	}

	// 存入键值对
	for i, field := range fields {
		value := values[i]
		dict.Put(field, value)
	}

	db.addAof(utils.ToCmdLine2("hmset", args...))
	return &reply.OKReply{}
}

// HMGET key field1 [field2]
func execHMGet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	size := len(args) - 1
	fields := make([]string, size)
	for i := 0; i < size; i++ {
		fields[i] = string(args[i+1])
	}

	// 获取dict结构
	result := make([][]byte, size)
	dict, errReply := db.getAsDict(key)
	if errReply != nil {
		return errReply
	}
	// 回复空
	if dict == nil {
		return reply.MakeMultiBulkReply(result)
	}
	for i, field := range fields {
		value, ok := dict.Get(field)
		if !ok {
			result[i] = nil
		} else {
			bytes, _ := value.([]byte)
			result[i] = bytes
		}
	}
	return reply.MakeMultiBulkReply(result)
}

// 获取dict的所有key
func execHKeys(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	dict, errReply := db.getAsDict(key)
	if errReply != nil {
		return errReply
	}
	if dict == nil {
		return &reply.EmptyMultiBulkReply{}
	}

	fields := make([][]byte, dict.Len())
	i := 0
	dict.ForEach(func(key string, val interface{}) bool {
		fields[i] = []byte(key)
		i++
		return true
	})
	return reply.MakeMultiBulkReply(fields[:i])
}

func execHVals(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	dict, errReply := db.getAsDict(key)
	if errReply != nil {
		return errReply
	}
	if dict == nil {
		return &reply.EmptyMultiBulkReply{}
	}

	values := make([][]byte, dict.Len())
	i := 0
	dict.ForEach(func(key string, val interface{}) bool {
		values[i], _ = val.([]byte)
		i++
		return true
	})
	return reply.MakeMultiBulkReply(values[:i])
}

func execHGetAll(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	// get entity
	dict, errReply := db.getAsDict(key)
	if errReply != nil {
		return errReply
	}
	if dict == nil {
		return &reply.EmptyMultiBulkReply{}
	}

	size := dict.Len()
	result := make([][]byte, size*2)
	i := 0
	dict.ForEach(func(key string, val interface{}) bool {
		result[i] = []byte(key)
		i++
		result[i], _ = val.([]byte)
		i++
		return true
	})
	return reply.MakeMultiBulkReply(result[:i])
}

func undoHSet(db *DB, args [][]byte) []CmdLine {
	key := string(args[0])
	field := string(args[1])
	return rollbackHashFields(db, key, field)
}

func undoHDel(db *DB, args [][]byte) []CmdLine {
	key := string(args[0])
	fields := make([]string, len(args)-1)
	fieldArgs := args[1:]
	for i, v := range fieldArgs {
		fields[i] = string(v)
	}
	return rollbackHashFields(db, key, fields...)
}

func undoHMSet(db *DB, args [][]byte) []CmdLine {
	key := string(args[0])
	size := (len(args) - 1) / 2
	fields := make([]string, size)
	for i := 0; i < size; i++ {
		fields[i] = string(args[2*i+1])
	}
	return rollbackHashFields(db, key, fields...)
}

func init() {
	// 插入一个键值对
	RegisterCommand("HSet", execHSet, writeFirstKey, undoHSet, 4)
	// 当key不存在时，插入一个键值对
	RegisterCommand("HSetNX", execHSetNX, writeFirstKey, undoHSet, 4)
	// 获取key对应的value
	RegisterCommand("HGet", execHGet, readFirstKey, nil, 3)
	// 判断key是否存在
	RegisterCommand("HExists", execHExists, readFirstKey, nil, 3)
	// 删除一个或多个键值对
	RegisterCommand("HDel", execHDel, writeFirstKey, undoHDel, -3)
	// 获取哈希表中的键值对个数
	RegisterCommand("HLen", execHLen, readFirstKey, nil, 2)
	// 插入数个键值对
	RegisterCommand("HMSet", execHMSet, writeFirstKey, undoHMSet, -4)
	// 获取数个key对应的value
	RegisterCommand("HMGet", execHMGet, readFirstKey, nil, -3)
	// 获取所有key
	RegisterCommand("HKeys", execHKeys, readFirstKey, nil, 2)
	// 获取所有value
	RegisterCommand("HVals", execHVals, readFirstKey, nil, 2)
	// 获取所有key-value
	RegisterCommand("HGetAll", execHGetAll, readFirstKey, nil, 2)
}
