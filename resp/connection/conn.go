package connection

import (
	"GoRedis/lib/sync/wait"
	"net"
	"sync"
	"time"
)

// Connection 协议层与每个客户端连接的描述
type Connection struct {
	conn net.Conn
	// 为了防止服务端被杀掉，在杀掉之前，需要把连接的客户端的所有服务处理完
	waitingReply wait.Wait
	// 操作一个客户端时上锁
	mu sync.Mutex
	// 切换数据库
	selectedDB int
}

func NewConn(conn net.Conn) *Connection {
	return &Connection{
		conn: conn,
	}
}

// RemoteAddr 返回远程主机地址
func (c *Connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Close 超时结束
func (c *Connection) Close() error {
	c.waitingReply.WaitWithTimeout(10 * time.Second)
	_ = c.conn.Close()
	return nil
}

// Write 向客户端发送回复
func (c *Connection) Write(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	// 同一时刻只能一个协程给客户端写数据
	c.mu.Lock()
	c.waitingReply.Add(1)
	defer func() {
		c.waitingReply.Done()
		c.mu.Unlock()
	}()

	_, err := c.conn.Write(b)
	return err
}

// GetDBIndex 返回当前在使用的数据库
func (c *Connection) GetDBIndex() int {
	return c.selectedDB
}

// SelectDB 选择数据库
func (c *Connection) SelectDB(dbNum int) {
	c.selectedDB = dbNum
}
