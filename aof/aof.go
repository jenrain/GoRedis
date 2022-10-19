package aof

import (
	"GoRedis/config"
	databaseface "GoRedis/interface/database"
	"GoRedis/lib/logger"
	"GoRedis/lib/utils"
	"GoRedis/resp/connection"
	"GoRedis/resp/parser"
	"GoRedis/resp/reply"
	"io"
	"os"
	"strconv"
)

// CmdLine 命令行
type CmdLine = [][]byte

const (
	aofQueueSize = 1 << 16
)

// 将指令和数据库编号封装起来
type payload struct {
	cmdLine CmdLine
	dbIndex int
}

// AofHandler 从通道接收消息并写入 AOF 文件
type AofHandler struct {
	database    databaseface.Database
	aofChan     chan *payload
	aofFile     *os.File
	aofFilename string
	currentDB   int
}

func NewAofHandler(database databaseface.Database) (*AofHandler, error) {
	handler := &AofHandler{}
	// 从配置文件中读取文件名
	handler.aofFilename = config.Properties.AppendFilename
	handler.database = database
	// 加载aof文件
	handler.LoadAof()
	aofFile, err := os.OpenFile(handler.aofFilename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	handler.aofFile = aofFile
	// 初始化通道
	handler.aofChan = make(chan *payload, aofQueueSize)
	// 开一个协程用于aof文件的落盘
	go func() {
		handler.handlerAof()
	}()
	return handler, nil
}

// AddAof 追加Aof文件，然后塞到channel中
func (handler *AofHandler) AddAof(dbIndex int, cmd CmdLine) {
	if config.Properties.AppendOnly && handler.aofChan != nil {
		handler.aofChan <- &payload{
			cmdLine: cmd,
			dbIndex: dbIndex,
		}
	}
}

// handlerAof aof文件落盘
func (handler *AofHandler) handlerAof() {
	handler.currentDB = 0
	for p := range handler.aofChan {
		// 需要切换数据库
		if p.dbIndex != handler.currentDB {
			data := reply.MakeMultiBulkReply(utils.ToCmdLine("select", strconv.Itoa(p.dbIndex))).ToBytes()
			_, err := handler.aofFile.Write(data)
			if err != nil {
				logger.Error(err)
				continue
			}
			handler.currentDB = p.dbIndex
		}
		data := reply.MakeMultiBulkReply(p.cmdLine).ToBytes()
		_, err := handler.aofFile.Write(data)
		if err != nil {
			logger.Error(err)
			continue
		}
	}
}

// LoadAof 加载Aof文件
func (handler *AofHandler) LoadAof() {
	// Open就是以只读的方式打开一个文件
	file, err := os.Open(handler.aofFilename)
	if err != nil {
		logger.Error(err)
		return
	}
	defer file.Close()
	ch := parser.ParseStream(file)
	// 创建一个伪客户端，然后把伪客户端传参给Exec函数，目的是获取dbIndex字段，其它字段其实是没有用的
	fackConn := &connection.Connection{}
	for p := range ch {
		if p.Err != nil {
			if p.Err == io.EOF {
				break
			}
			logger.Error(p.Err)
			continue
		}
		if p.Data == nil {
			logger.Error("empty payload")
			continue
		}
		// aof文件中记录的只能是 reply.MultiBulkReply 类型的数据
		r, ok := p.Data.(*reply.MultiBulkReply)
		if !ok {
			logger.Error("need multi mulk")
			continue
		}
		rep := handler.database.Exec(fackConn, r.Args)
		if reply.IsErrorReply(rep) {
			logger.Error(rep)
		}
	}
}
