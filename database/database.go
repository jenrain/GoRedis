package database

import (
	"GoRedis/aof"
	"GoRedis/config"
	"GoRedis/interface/resp"
	"GoRedis/lib/logger"
	"GoRedis/resp/reply"
	"strconv"
	"strings"
)

// StandaloneDatabase Redis内核数据库
// 是一个装有数据库map的切片
type StandaloneDatabase struct {
	dbSet      []*DB
	aofHandler *aof.AofHandler
}

func NewStandaloneDatabase() *StandaloneDatabase {
	database := &StandaloneDatabase{}
	if config.Properties.Databases <= 0 {
		config.Properties.Databases = 16
	}
	database.dbSet = make([]*DB, config.Properties.Databases)
	for i := range database.dbSet {
		db := makeDB()
		db.index = i
		database.dbSet[i] = db
	}
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
func (database *StandaloneDatabase) Exec(client resp.Connection, args [][]byte) resp.Reply {
	// 防止突然终止程序
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()
	cmdName := strings.ToLower(string(args[0]))
	if cmdName == "select" {
		// 发送的参数个数有问题
		if len(args) != 2 {
			return reply.MakeArgNumErrReply("select")
		}
		return execSelect(client, database, args[1:])
	}
	dbIndex := client.GetDBIndex()
	db := database.dbSet[dbIndex]
	return db.Exec(client, args)
}

func (database *StandaloneDatabase) AfterClientClose(c resp.Connection) {

}

func (database *StandaloneDatabase) Close() {

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
