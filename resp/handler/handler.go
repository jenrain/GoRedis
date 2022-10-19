package handler

/*
 * 封装一个协议层的TCP Handler
 */

import (
	"GoRedis/database"
	databaseface "GoRedis/interface/database"
	"GoRedis/lib/logger"
	"GoRedis/lib/sync/atomic"
	"GoRedis/resp/connection"
	"GoRedis/resp/parser"
	"GoRedis/resp/reply"
	"context"
	"io"
	"net"
	"strings"
	"sync"
)

var (
	unknownErrReplyBytes = []byte("-ERR unknown\r\n")
)

// RespHandler 实现 tcp.Handler 并充当 redis 处理程序
type RespHandler struct {
	activeConn sync.Map
	db         databaseface.Database
	closing    atomic.Boolean
}

func MakeHandler() *RespHandler {
	var db databaseface.Database
	db = database.NewStandaloneDatabase()
	return &RespHandler{
		db: db,
	}
}

// 关闭一个客户端的连接
func (h *RespHandler) closeClient(client *connection.Connection) {
	_ = client.Close()
	h.db.AfterClientClose(client)
	h.activeConn.Delete(client)
}

// Handle 接收并执行redis命令
func (h *RespHandler) Handle(ctx context.Context, conn net.Conn) {
	if h.closing.Get() {
		// 关闭处理程序拒绝新连接
		_ = conn.Close()
	}

	client := connection.NewConn(conn)
	h.activeConn.Store(client, 1)

	// 程序会为每一个客户端创建一个协程来解析
	ch := parser.ParseStream(conn)

	// 监听管道
	// 管道中的payload只有 错误 或者 解析好的命令
	for payload := range ch {
		if payload.Err != nil {
			// 什么都没有读到，读取结束
			if payload.Err == io.EOF ||
				// 读到了内容，但是读的内容小于min，说明读的过程中出现了意外中断
				payload.Err == io.ErrUnexpectedEOF ||
				// 使用一个已经关闭的客户端
				strings.Contains(payload.Err.Error(), "use of closed network connection") {
				// 直接关闭客户端
				h.closeClient(client)
				logger.Info("connection closed: " + client.RemoteAddr().String())
				return
			}
			// 协议错误
			errReply := reply.MakeErrReply(payload.Err.Error())
			// 将错误写回给客户端
			err := client.Write(errReply.ToBytes())
			if err != nil {
				h.closeClient(client)
				logger.Info("connection closed: " + client.RemoteAddr().String())
				return
			}
			// 继续监听通道
			continue
		}
		// 获取的消息为空
		if payload.Data == nil {
			logger.Error("empty payload")
			continue
		}
		// Data只是一个接口，需要转化为二维字节数组
		r, ok := payload.Data.(*reply.MultiBulkReply)
		if !ok {
			logger.Error("require multi bulk reply")
			continue
		}
		// 执行命令
		result := h.db.Exec(client, r.Args)
		if result != nil {
			_ = client.Write(result.ToBytes())
			// 结果为空，只能是未知错误
		} else {
			_ = client.Write(unknownErrReplyBytes)
		}
	}
}

// Close 关闭整个协议层
func (h *RespHandler) Close() error {
	logger.Info("handler shutting down...")
	h.closing.Set(true)
	h.activeConn.Range(func(key interface{}, val interface{}) bool {
		client := key.(*connection.Connection)
		_ = client.Close()
		return true
	})
	h.db.Close()
	return nil
}
