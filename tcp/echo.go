package tcp

import (
	"GoRedis/lib/logger"
	"GoRedis/lib/sync/atomic"
	"GoRedis/lib/sync/wait"
	"bufio"
	"context"
	"io"
	"net"
	"sync"
	"time"
)

// EchoClient 封装客户端信息
type EchoClient struct {
	Conn    net.Conn
	Waiting wait.Wait
}

func MakeHandler() *EchoHandler {
	return &EchoHandler{}
}

func (e *EchoClient) Close() error {
	// 等待十秒没有结束就关闭
	e.Waiting.WaitWithTimeout(10 * time.Second)
	_ = e.Conn.Close()
	return nil
}

// EchoHandler 封装处理客户端请求的业务
type EchoHandler struct {
	// 记录有多少个客户端连接进来
	activeConn sync.Map
	// 记录现在的业务是否正在被关闭
	closing atomic.Boolean
}

func (handler *EchoHandler) Handle(ctx context.Context, conn net.Conn) {
	// 判断现在的业务是否正在被关闭
	if handler.closing.Get() {
		_ = conn.Close()
	}
	// 开始处理进来的客户端
	// 先包装成一个 EchoClient
	client := &EchoClient{
		Conn: conn,
	}
	// 储存到map中
	// map当set用
	handler.activeConn.Store(client, struct{}{})

	reader := bufio.NewReader(conn)
	for {
		// 以换行符为分隔符读取字符
		msg, err := reader.ReadString('\n')
		if err != nil {
			// 消息读完
			if err == io.EOF {
				logger.Info("connecting close")
				handler.activeConn.Delete(client)
			} else {
				logger.Warn(err)
			}
			return
		}
		// 定义一个回写的业务
		client.Waiting.Add(1)
		b := []byte(msg)
		_, err = conn.Write(b)
		client.Waiting.Done()
	}
}

func (handler *EchoHandler) Close() error {
	logger.Info("handler shutting down")
	handler.closing.Set(true)
	// 将储存在set中的客户端一个一个关闭
	handler.activeConn.Range(func(key, value interface{}) bool {
		client := key.(*EchoClient)
		_ = client.Conn.Close()
		// 返回true代表将该方法施加到下一个key上
		return true
	})
	return nil
}
