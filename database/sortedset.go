package database

import (
	SortedSet "GoRedis/datastruct/sortedset"
	"GoRedis/interface/database"
	"GoRedis/interface/resp"
	"GoRedis/lib/utils"
	"GoRedis/resp/reply"
	"strconv"
	"strings"
)

func (db *DB) getAsSortedSet(key string) (*SortedSet.SortedSet, reply.ErrorReply) {
	// 获取key对应的有序集合结构
	entity, exists := db.GetEntity(key)
	if !exists {
		return nil, nil
	}
	// 转化为有序集合
	sortedSet, ok := entity.Data.(*SortedSet.SortedSet)
	if !ok {
		return nil, &reply.WrongTypeErrReply{}
	}
	return sortedSet, nil
}

func (db *DB) getOrInitSortedSet(key string) (sortedSet *SortedSet.SortedSet, inited bool, errReply reply.ErrorReply) {
	sortedSet, errReply = db.getAsSortedSet(key)
	if errReply != nil {
		return nil, false, errReply
	}
	inited = false
	// 需要创建
	if sortedSet == nil {
		sortedSet = SortedSet.Make()
		db.PutEntity(key, &database.DataEntity{
			Data: sortedSet,
		})
		inited = true
	}
	return sortedSet, inited, nil
}

// ZADD key score1 member1 [score2 member2]
func execZAdd(db *DB, args [][]byte) resp.Reply {
	// 语法错误
	if len(args)%2 != 1 {
		return reply.MakeSyntaxErrReply()
	}
	key := string(args[0])
	size := (len(args) - 1) / 2
	// 储存key-score对组，方便后面存入sortedset
	elements := make([]*SortedSet.Element, size)
	for i := 0; i < size; i++ {
		// 取出分数和成员
		scoreValue := args[2*i+1]
		member := string(args[2*i+2])
		score, err := strconv.ParseFloat(string(scoreValue), 64)
		if err != nil {
			return reply.MakeErrReply("ERR value is not a valid float")
		}
		elements[i] = &SortedSet.Element{
			Member: member,
			Score:  score,
		}
	}

	// 获取或者初始化key对应的sortedset
	sortedSet, _, errReply := db.getOrInitSortedSet(key)
	if errReply != nil {
		return errReply
	}

	// 添加元素进sortedset
	i := 0
	for _, e := range elements {
		if sortedSet.Add(e.Member, e.Score) {
			i++
		}
	}

	db.addAof(utils.ToCmdLine2("zadd", args...))
	return reply.MakeIntReply(int64(i))
}

// ZSCORE key member
func execZScore(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])
	member := string(args[1])

	// 获取key对应的sortedset
	sortedSet, errReply := db.getAsSortedSet(key)
	if errReply != nil {
		return errReply
	}
	if sortedSet == nil {
		return reply.NullBulkReply{}
	}

	// 获取sortedset中的成员
	element, exists := sortedSet.Get(member)
	if !exists {
		return &reply.NullBulkReply{}
	}
	// 转换分数
	value := strconv.FormatFloat(element.Score, 'f', -1, 64)
	return reply.MakeBulkReply([]byte(value))
}

// ZRANK key member
func execZRank(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])
	member := string(args[1])

	// 获取key对应的sortedset
	sortedSet, errReply := db.getAsSortedSet(key)
	if errReply != nil {
		return errReply
	}
	if sortedSet == nil {
		return reply.NullBulkReply{}
	}

	rank := sortedSet.GetRank(member, false)
	if rank < 0 {
		return &reply.NullBulkReply{}
	}
	return reply.MakeIntReply(int64(rank))
}

// ZCOUNT key min max
func execZCount(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])

	// 封装左边界
	min, err := SortedSet.ParseScoreBorder(string(args[1]))
	if err != nil {
		return reply.MakeErrReply(err.Error())
	}

	// 封装右边界
	max, err := SortedSet.ParseScoreBorder(string(args[2]))
	if err != nil {
		return reply.MakeErrReply(err.Error())
	}

	// 获取key对应的sortedset
	sortedSet, errReply := db.getAsSortedSet(key)
	if errReply != nil {
		return errReply
	}
	if sortedSet == nil {
		return reply.MakeIntReply(int64(0))
	}

	return reply.MakeIntReply(sortedSet.Count(min, max))

}

// ZCARD key
func execZCard(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])

	// 获取key对应的sortedset
	sortedSet, errReply := db.getAsSortedSet(key)
	if errReply != nil {
		return errReply
	}
	if sortedSet == nil {
		return reply.MakeIntReply(int64(0))
	}

	return reply.MakeIntReply(sortedSet.Len())
}

// 返回索引区间处于[start, stop]的成员
func range0(db *DB, key string, start int64, stop int64, withScores bool, desc bool) resp.Reply {
	// 从数据库获取key对应的sortedset
	sortedSet, errReply := db.getAsSortedSet(key)
	if errReply != nil {
		return errReply
	}
	if sortedSet == nil {
		return &reply.EmptyMultiBulkReply{}
	}

	// 处理索引值
	size := sortedSet.Len()
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

	// 以切片形式返回处于[start, stop]中的元素
	slice := sortedSet.Range(start, stop, desc)
	// 返回结果需要带上分数
	if withScores {
		result := make([][]byte, len(slice)*2)
		i := 0
		for _, element := range slice {
			result[i] = []byte(element.Member)
			i++
			scoreStr := strconv.FormatFloat(element.Score, 'f', -1, 64)
			result[i] = []byte(scoreStr)
			i++
		}
		return reply.MakeMultiBulkReply(result)
	}
	result := make([][]byte, len(slice))
	i := 0
	for _, element := range slice {
		result[i] = []byte(element.Member)
		i++
	}
	return reply.MakeMultiBulkReply(result)
}

// ZRANGE key start stop [WITHSCORES]
func execZRange(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	// 参数只能是3个或者4个
	if len(args) != 3 && len(args) != 4 {
		return reply.MakeErrReply("ERR wrong number of arguments for 'zrange' command")
	}
	// 标记参数带不带 WITHSCORES
	withScores := false
	// 参数个数为4个时，最后一个参数必为 WITHSCORES
	if len(args) == 4 {
		if strings.ToUpper(string(args[3])) != "WITHSCORES" {
			return reply.MakeErrReply("syntax error")
		}
		withScores = true
	}
	// 获取数据库的key
	key := string(args[0])
	// 获取start和stop
	start, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}
	stop, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}
	return range0(db, key, start, stop, withScores, false)
}

// ZREM key member [member...]
func execZRem(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])
	fields := make([]string, len(args)-1)
	fieldArgs := args[1:]
	for i, v := range fieldArgs {
		fields[i] = string(v)
	}

	// 获取key对应的sortedset
	sortedSet, errReply := db.getAsSortedSet(key)
	if errReply != nil {
		return errReply
	}
	if sortedSet == nil {
		return reply.MakeIntReply(0)
	}

	var deleted int64 = 0
	for _, field := range fields {
		if sortedSet.Remove(field) {
			deleted++
		}
	}
	if deleted > 0 {
		db.addAof(utils.ToCmdLine2("zrem", args...))
	}
	return reply.MakeIntReply(deleted)
}

// ZREMRANGEBYSCORE key min max
func execZRemRangeByScore(db *DB, args [][]byte) resp.Reply {
	// 参数固定是3个
	if len(args) != 3 {
		return reply.MakeErrReply("ERR wrong number of arguments for 'zremrangebyscore' command")
	}
	key := string(args[0])

	// 获取并封装左边界和右边界
	min, err := SortedSet.ParseScoreBorder(string(args[1]))
	if err != nil {
		return reply.MakeErrReply(err.Error())
	}

	max, err := SortedSet.ParseScoreBorder(string(args[2]))
	if err != nil {
		return reply.MakeErrReply(err.Error())
	}

	// 获取key对应的sortedset
	sortedSet, errReply := db.getAsSortedSet(key)
	if errReply != nil {
		return errReply
	}
	if sortedSet == nil {
		return &reply.EmptyMultiBulkReply{}
	}

	removed := sortedSet.RemoveByScore(min, max)
	if removed > 0 {
		db.addAof(utils.ToCmdLine2("zremrangebyscore", args...))
	}
	return reply.MakeIntReply(removed)
}

// ZREMRANGEBYRANK key start stop
func execZRemRangeByRank(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	// 获取key
	key := string(args[0])

	// 获取start和stop
	start, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}
	stop, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}

	// 获取key对应的sortedset
	sortedSet, errReply := db.getAsSortedSet(key)
	if errReply != nil {
		return errReply
	}
	if sortedSet == nil {
		return reply.MakeIntReply(0)
	}

	// 处理index
	size := sortedSet.Len()
	if start < -1*size {
		start = 0
	} else if start < 0 {
		start = size + start
	} else if start >= size {
		return reply.MakeIntReply(0)
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

	removed := sortedSet.RemoveByRank(start, stop)
	if removed > 0 {
		db.addAof(utils.ToCmdLine2("zremrangebyrank", args...))
	}
	return reply.MakeIntReply(removed)
}

func init() {
	// 插入一个成员
	RegisterCommand("ZAdd", execZAdd, -4)
	// 返回成员的分数值
	RegisterCommand("ZScore", execZScore, 3)
	// 返回成员的排名
	RegisterCommand("ZRank", execZRank, 3)
	// 返回处于给定分数区间的成员数
	RegisterCommand("ZCount", execZCount, 4)
	// 返回集合的所有成员
	RegisterCommand("ZCard", execZCard, 2)
	// 通过索引区间返回指定区间内的成员
	RegisterCommand("ZRange", execZRange, -4)
	// 移除有序集合中的一个或多个成员
	RegisterCommand("ZRem", execZRem, -3)
	// 移除有序集合中给定的分数区间的所有成员
	RegisterCommand("ZRemRangeByScore", execZRemRangeByScore, 4)
	// 移除有序集合中给定的排名区间的所有成员
	RegisterCommand("ZRemRangeByRank", execZRemRangeByRank, 4)
}
