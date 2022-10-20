package database

import (
	HashSet "GoRedis/datastruct/set"
	"GoRedis/interface/database"
	"GoRedis/interface/resp"
	"GoRedis/lib/utils"
	"GoRedis/resp/reply"
)

func (db *DB) getAsSet(key string) (*HashSet.Set, reply.ErrorReply) {
	entity, exists := db.GetEntity(key)
	// 不存在该set结构
	if !exists {
		return nil, nil
	}
	set, ok := entity.Data.(*HashSet.Set)
	if !ok {
		return nil, &reply.WrongTypeErrReply{}
	}
	return set, nil
}

// inited 表示是否创建了新的set结构
func (db *DB) getOrInitSet(key string) (set *HashSet.Set, inited bool, errReply reply.ErrorReply) {
	set, errReply = db.getAsSet(key)
	if errReply != nil {
		return nil, false, errReply
	}

	inited = false
	if set == nil {
		set = HashSet.Make()
		db.PutEntity(key, &database.DataEntity{
			Data: set,
		})
		inited = true
	}
	return set, inited, nil
}

// SADD key member1 [member2]
func execSAdd(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	members := args[1:]

	// 获取或者初始化set结构
	set, _, errReply := db.getOrInitSet(key)
	if errReply != nil {
		return errReply
	}
	counter := 0
	for _, member := range members {
		// 修改返回0，插入新成员返回1
		counter += set.Add(string(member))
	}
	db.addAof(utils.ToCmdLine2("sadd", args...))
	return reply.MakeIntReply(int64(counter))
}

// SISMEMBER key member
func execSIsMember(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	member := string(args[1])

	// 获取set
	set, errReply := db.getAsSet(key)
	if errReply != nil {
		return errReply
	}
	if set == nil {
		return reply.MakeIntReply(int64(0))
	}

	has := set.Has(member)
	if has {
		return reply.MakeIntReply(int64(1))
	}
	return reply.MakeIntReply(int64(0))
}

// SREM key member1 [member2]
func execSRem(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	members := args[1:]

	set, errReply := db.getAsSet(key)
	if errReply != nil {
		return errReply
	}
	if set == nil {
		return reply.MakeIntReply(int64(0))
	}

	counter := 0
	for _, member := range members {
		counter += set.Remove(string(member))
	}

	// 集合中已经没有成员
	if set.Len() == 0 {
		db.Remove(key)
	}

	if counter > 0 {
		db.addAof(utils.ToCmdLine2("srem", args...))
	}
	return reply.MakeIntReply(int64(counter))
}

//SCARD key
func execSCard(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	set, errReply := db.getAsSet(key)
	if errReply != nil {
		return errReply
	}

	if set == nil {
		return reply.MakeIntReply(int64(0))
	}

	return reply.MakeIntReply(int64(set.Len()))
}

// 返回集合的所有成员
func execSMembers(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	set, errReply := db.getAsSet(key)
	if errReply != nil {
		return errReply
	}
	if set == nil {
		return &reply.EmptyMultiBulkReply{}
	}

	arr := make([][]byte, set.Len())
	i := 0
	set.ForEach(func(member string) bool {
		arr[i] = []byte(member)
		i++
		return true
	})
	return reply.MakeMultiBulkReply(arr)
}

// SINTER key1 [key2]
func execSInter(db *DB, args [][]byte) resp.Reply {
	keys := make([]string, len(args))
	// 将要操作的集合都取出来
	for i, arg := range args {
		keys[i] = string(arg)
	}

	var result *HashSet.Set
	for _, key := range keys {
		set, errReply := db.getAsSet(key)
		if errReply != nil {
			return errReply
		}
		if set == nil {
			return &reply.EmptyMultiBulkReply{}
		}

		if result == nil {
			// 初始化
			result = HashSet.Make(set.ToSlice()...)
		} else {
			result = result.Intersect(set)
			if result.Len() == 0 {
				return &reply.EmptyMultiBulkReply{}
			}
		}
	}

	// 把result集合中的所有元素装进arr中返回给客户端
	arr := make([][]byte, result.Len())
	i := 0
	result.ForEach(func(member string) bool {
		arr[i] = []byte(member)
		i++
		return true
	})
	return reply.MakeMultiBulkReply(arr)
}

// SUNION key1 [key2]
func execSUnion(db *DB, args [][]byte) resp.Reply {
	keys := make([]string, len(args))
	// 将要操作的集合都取出来
	for i, arg := range args {
		keys[i] = string(arg)
	}

	var result *HashSet.Set
	for _, key := range keys {
		set, errReply := db.getAsSet(key)
		if errReply != nil {
			return errReply
		}
		if set == nil {
			continue
		}

		if result == nil {
			// 初始化
			result = HashSet.Make(set.ToSlice()...)
		} else {
			result = result.Union(set)
		}
	}

	if result == nil {
		return &reply.EmptyMultiBulkReply{}
	}
	// 把result集合中的所有元素装进arr中返回给客户端
	arr := make([][]byte, result.Len())
	i := 0
	result.ForEach(func(member string) bool {
		arr[i] = []byte(member)
		i++
		return true
	})
	return reply.MakeMultiBulkReply(arr)
}

// SDIFF key1 [key2]
func execSDiff(db *DB, args [][]byte) resp.Reply {
	keys := make([]string, len(args))
	// 将要操作的集合都取出来
	for i, arg := range args {
		keys[i] = string(arg)
	}

	var result *HashSet.Set
	for i, key := range keys {
		set, errReply := db.getAsSet(key)
		if errReply != nil {
			return errReply
		}
		if set == nil {
			// 左边的集合不能为空
			if i == 0 {
				return &reply.EmptyMultiBulkReply{}
			}
			continue
		}

		if result == nil {
			// 初始化
			result = HashSet.Make(set.ToSlice()...)
		} else {
			result = result.Diff(set)
			if result.Len() == 0 {
				return &reply.EmptyMultiBulkReply{}
			}
		}
	}

	if result == nil {
		return &reply.EmptyMultiBulkReply{}
	}

	// 把result集合中的所有元素装进arr中返回给客户端
	arr := make([][]byte, result.Len())
	i := 0
	result.ForEach(func(member string) bool {
		arr[i] = []byte(member)
		i++
		return true
	})
	return reply.MakeMultiBulkReply(arr)
}

func init() {
	// 插入一个成员
	RegisterCommand("SAdd", execSAdd, writeFirstKey, undoSetChange, -3)
	// 判断给定参数是否是集合的成员
	RegisterCommand("SIsMember", execSIsMember, readFirstKey, nil, 3)
	// 删除集合的一个或多个成员
	RegisterCommand("SRem", execSRem, writeFirstKey, undoSetChange, -3)
	// 返回集合的成员数量
	RegisterCommand("SCard", execSCard, readFirstKey, nil, 2)
	// 返回集合的所有成员
	RegisterCommand("SMembers", execSMembers, readFirstKey, nil, 2)
	// 交集
	RegisterCommand("SInter", execSInter, prepareSetCalculate, nil, -2)
	// 并集
	RegisterCommand("SUnion", execSUnion, prepareSetCalculate, nil, -2)
	// 差集
	RegisterCommand("SDiff", execSDiff, prepareSetCalculate, nil, -2)
}
