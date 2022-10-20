package database

import (
	"GoRedis/aof"
	"GoRedis/config"
	"GoRedis/interface/resp"
	"GoRedis/lib/logger"
	"GoRedis/pubsub"
	"GoRedis/resp/reply"
	"strconv"
	"strings"
)

// StandaloneDatabase Redis内核数据库
// 是一个装有数据库map的切片
type StandaloneDatabase struct {
	dbSet []*DB
	// 处理aof持久化
	aofHandler *aof.AofHandler
	// 处理发布/订阅
	hub *pubsub.Hub
}

func NewStandaloneDatabase() *StandaloneDatabase {
	database := &StandaloneDatabase{}
	if config.Properties.Databases <= 0 {
		config.Properties.Databases = 16
	}
	database.dbSet = make([]*DB, config.Properties.Databases)
	database.hub = pubsub.MakeHub()
	// 初始化所有DB
	for i := range database.dbSet {
		db := makeDB()
		db.index = i
		database.dbSet[i] = db
	}
	// 初始化aof持久化
	if config.Properties.AppendOnly {
		aofHandler, err := aof.NewAofHandler(database)
		if err != nil {
			panic(err)
		}
		database.aofHandler = aofHandler
		for _, db := range database.dbSet {
			// 防止闭包出现问题
			sdb := db
			sdb.addAof = func(line CmdLine) {
				database.aofHandler.AddAof(sdb.index, line)
			}
		}
	}
	return database
}

// Exec
// set k v
// get k
// select 2
func (Sdb *StandaloneDatabase) Exec(client resp.Connection, cmdLine [][]byte) resp.Reply {
	// 防止突然终止程序
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()
	// 获取第一个命令的名称
	cmdName := strings.ToLower(string(cmdLine[0]))

	if cmdName == "subscribe" {
		if len(cmdLine) < 2 {
			return reply.MakeArgNumErrReply("subscribe")
		}
		return pubsub.Subscribe(Sdb.hub, client, cmdLine[1:])
	} else if cmdName == "publish" {
		return pubsub.Publish(Sdb.hub, cmdLine[1:])
	} else if cmdName == "unsubscribe" {
		return pubsub.UnSubscribe(Sdb.hub, client, cmdLine[1:])
	} else if cmdName == "flushdb" {
		if !validateArity(1, cmdLine) {
			return reply.MakeArgNumErrReply(cmdName)
		}
		// 事务状态下无法执行flushdb命令
		if client.InMultiState() {
			return reply.MakeErrReply("ERR command 'FlushDB' cannot be used in MULTI")
		}
		return Sdb.flushDB(client.GetDBIndex())
	}

	dbIndex := client.GetDBIndex()
	db := Sdb.dbSet[dbIndex]
	return db.Exec(client, cmdLine)
}

func (Sdb *StandaloneDatabase) AfterClientClose(c resp.Connection) {

}

func (Sdb *StandaloneDatabase) Close() {

}

// select 2
func execSelect(c resp.Connection, database *StandaloneDatabase, args [][]byte) resp.Reply {
	dbIndex, err := strconv.Atoi(string(args[0]))
	// select a
	if err != nil {
		return reply.MakeErrReply("ERR invalid DB index")
	}
	// select 65536
	if dbIndex > len(database.dbSet) {
		return reply.MakeErrReply("ERR DB index is out of range")
	}
	c.SelectDB(dbIndex)
	return reply.MakeOkReply()
}

// 清除一个db
func (Sdb *StandaloneDatabase) flushDB(dbIndex int) resp.Reply {
	if dbIndex >= len(Sdb.dbSet) || dbIndex < 0 {
		return reply.MakeErrReply("ERR DB index is out of range")
	}
	newDB := makeDB()
	Sdb.loadDB(dbIndex, newDB)
	return &reply.OKReply{}
}

func (Sdb *StandaloneDatabase) loadDB(dbIndex int, newDB *DB) resp.Reply {
	if dbIndex >= len(Sdb.dbSet) || dbIndex < 0 {
		return reply.MakeErrReply("ERR DB index is out of range")
	}
	oldDB, _ := Sdb.selectDB(dbIndex)
	newDB.index = dbIndex
	newDB.addAof = oldDB.addAof
	Sdb.dbSet[dbIndex] = newDB
	return &reply.OKReply{}
}

// 根据下标返回db
func (Sdb *StandaloneDatabase) selectDB(dbIndex int) (*DB, *reply.StandardErrReply) {
	if dbIndex >= len(Sdb.dbSet) || dbIndex < 0 {
		return nil, reply.MakeErrReply("ERR DB index is out of range")
	}
	return Sdb.dbSet[dbIndex], nil
}
