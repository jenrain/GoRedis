package database

import (
	List "GoRedis/datastruct/list"
	"GoRedis/interface/database"
	"GoRedis/interface/resp"
	"GoRedis/lib/utils"
	"GoRedis/resp/reply"
	"strconv"
)

// 获取数据库中键对应的列表结构
func (db *DB) getAsList(key string) (List.List, reply.ErrorReply) {
	entity, ok := db.GetEntity(key)
	if !ok {
		return nil, nil
	}
	list, ok := entity.Data.(List.List)
	if !ok {
		return nil, &reply.WrongTypeErrReply{}
	}
	return list, nil
}

// 调用getAsList，如果得到的list为nil，就创建一个
func (db *DB) getOrInitList(key string) (list List.List, isNew bool, errReply reply.ErrorReply) {
	list, errReply = db.getAsList(key)
	if errReply != nil {
		return nil, false, errReply
	}
	isNew = false
	// 需要创建
	if list == nil {
		// 使用快速列表
		list = List.NewQuickList()
		db.PutEntity(key, &database.DataEntity{Data: list})
		isNew = true
	}
	return list, isNew, nil
}

// LPush k v1 v2 ...
func execLPush(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	values := args[1:]
	// 获取或者初始化list
	list, _, errReply := db.getOrInitList(key)
	if errReply != nil {
		return errReply
	}
	// 插入元素
	for _, value := range values {
		list.Insert(0, value)
	}
	db.addAof(utils.ToCmdLine2("lpush", args...))
	return reply.MakeIntReply(int64(list.Len()))
}

// LPushX k v1 v2 ...
func execLPushX(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	values := args[1:]

	// 获取list
	list, errReply := db.getAsList(key)
	if errReply != nil {
		return errReply
	}
	if list == nil {
		return reply.MakeIntReply(int64(0))
	}

	// 如果存在就插入到list中
	for _, value := range values {
		list.Insert(0, value)
	}
	db.addAof(utils.ToCmdLine2("lpushx", args...))
	return reply.MakeIntReply(int64(list.Len()))
}

func execRPush(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	values := args[1:]

	// 获取或初始化list
	list, _, errReply := db.getOrInitList(key)
	if errReply != nil {
		return errReply
	}

	for _, value := range values {
		list.Add(value)
	}

	db.addAof(utils.ToCmdLine2("rpush", args...))
	return reply.MakeIntReply(int64(list.Len()))
}

func execRPushX(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	values := args[1:]

	// 获取list
	list, errReply := db.getAsList(key)
	if errReply != nil {
		return errReply
	}

	if list == nil {
		return reply.MakeIntReply(int64(0))
	}

	for _, value := range values {
		list.Add(value)
	}

	db.addAof(utils.ToCmdLine2("rpush", args...))
	return reply.MakeIntReply(int64(list.Len()))
}

// LPop key
func execLPop(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	// 获取list
	list, errReply := db.getAsList(key)
	if errReply != nil {
		return errReply
	}

	if list == nil {
		return &reply.NullBulkReply{}
	}

	val, _ := list.Remove(0).([]byte)
	// 如果删除的是最后一个元素
	if list.Len() == 0 {
		db.Remove(key)
	}

	db.addAof(utils.ToCmdLine2("lpop", args...))
	return reply.MakeBulkReply(val)
}

// RPop key
func execRPop(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	// 获取list
	list, errReply := db.getAsList(key)
	if errReply != nil {
		return errReply
	}

	if list == nil {
		return &reply.NullBulkReply{}
	}

	val, _ := list.RemoveLast().([]byte)
	// 如果删除的是最后一个元素
	if list.Len() == 0 {
		db.Remove(key)
	}

	db.addAof(utils.ToCmdLine2("rpop", args...))
	return reply.MakeBulkReply(val)
}

// LRem key count value
func execLRem(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	count64, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}
	count := int(count64)
	value := args[2]

	// 获取list
	list, errReply := db.getAsList(key)
	if errReply != nil {
		return errReply
	}
	if list == nil {
		return reply.MakeIntReply(int64(0))
	}

	var removed int
	if count == 0 {
		removed = list.RemoveAllByVal(func(a interface{}) bool {
			return utils.Equals(a, value)
		})
	} else if count > 0 {
		removed = list.RemoveByVal(func(a interface{}) bool {
			return utils.Equals(a, value)
		}, count)
	} else {
		removed = list.ReverseRemoveByVal(func(a interface{}) bool {
			return utils.Equals(a, value)
		}, -count)
	}

	if list.Len() == 0 {
		db.Remove(key)
	}

	if removed > 0 {
		db.addAof(utils.ToCmdLine2("lrem", args...))
	}

	return reply.MakeIntReply(int64(removed))
}

// LLEN key
func execLLen(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	list, errReply := db.getAsList(key)
	if errReply != nil {
		return errReply
	}
	if list == nil {
		return reply.MakeIntReply(int64(0))
	}
	size := int64(list.Len())
	return reply.MakeIntReply(size)
}

// LIndex key index
func execLIndex(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	index64, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}
	index := int(index64)

	list, errReply := db.getAsList(key)
	if errReply != nil {
		return errReply
	}
	if list == nil {
		return &reply.NullBulkReply{}
	}

	size := list.Len()
	if index < -1*size {
		return &reply.NullBulkReply{}
	} else if index < 0 {
		index = size + index
	} else if index >= size {
		return &reply.NullBulkReply{}
	}

	val, _ := list.Get(index).([]byte)
	return reply.MakeBulkReply(val)
}

// LSet key index value
func execLSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	index64, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}
	index := int(index64)
	value := args[2]

	list, errReply := db.getAsList(key)
	if errReply != nil {
		return errReply
	}
	if list == nil {
		return reply.MakeErrReply("ERR no such key")
	}

	// 检查边界
	size := list.Len()
	if index < -1*size {
		return reply.MakeErrReply("ERR index out of range")
	} else if index < 0 {
		index = size + index
	} else if index >= size {
		return reply.MakeErrReply("ERR index out of range")
	}

	list.Set(index, value)
	db.addAof(utils.ToCmdLine2("lset", args...))
	return &reply.OKReply{}
}

// LRANGE key start stop
func execLRange(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	start64, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}
	start := int(start64)
	stop64, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}
	stop := int(stop64)

	list, errReply := db.getAsList(key)
	if errReply != nil {
		return errReply
	}
	if list == nil {
		return &reply.EmptyMultiBulkReply{}
	}

	// 检查边界
	size := list.Len()
	if start < -1*size {
		start = 0
	} else if start < 0 {
		start = size + start
	} else if start >= size {
		return &reply.EmptyMultiBulkReply{}
	}

	if stop < -1*size {
		stop = 0
	} else if stop < 0 {
		stop = size + stop + 1
	} else if stop < size {
		stop = stop + 1
	} else {
		stop = size
	}
	if stop < start {
		stop = start
	}

	slice := list.Range(start, stop)
	result := make([][]byte, len(slice))
	for i, raw := range slice {
		bytes, _ := raw.([]byte)
		result[i] = bytes
	}
	return reply.MakeMultiBulkReply(result)
}

func init() {
	// 头插
	RegisterCommand("LPush", execLPush, -3)
	RegisterCommand("LPushX", execLPushX, -3)
	// 尾插
	RegisterCommand("RPush", execRPush, -3)
	RegisterCommand("RPushX", execRPushX, -3)
	// 弹出头部元素
	RegisterCommand("LPop", execLPop, 2)
	// 弹出尾部元素
	RegisterCommand("RPop", execRPop, 2)
	// 根据count删除元素
	// count == 0 删除等于value的所有元素
	// count > 0  顺序删除count个value元素
	// count < 0  逆序删除-count个value元素
	RegisterCommand("LRem", execLRem, 4)
	// 获取列表长度
	RegisterCommand("LLen", execLLen, 2)
	// 根据下标获取元素
	RegisterCommand("LIndex", execLIndex, 3)
	// 设置指定下标处的值为value（覆盖原值）
	RegisterCommand("LSet", execLSet, 4)
	// 返回指定区间的元素（返回一个切片）
	RegisterCommand("LRange", execLRange, 4)
}
