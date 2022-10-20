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

	/*
	 * 事务相关
	 */
	// 事务是否在执行
	multiState bool
	// 储存事务执行时的命令
	queue [][][]byte
	// 储存事务执行时被监视的key
	watching map[string]uint32
	// 储存事务执行时产生的错误
	txErrors []error

	/*
	 * 发布订阅相关
	 */
	// 订阅的频道
	subs map[string]bool
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

/*
 * 事务相关
 */

// InMultiState 返回事务此刻的状态
func (c *Connection) InMultiState() bool {
	return c.multiState
}

// SetMultiState 设置事务此刻的状态
func (c *Connection) SetMultiState(state bool) {
	if !state {
		c.watching = nil
		c.queue = nil
	}
	c.multiState = state
}

// GetQueuedCmdLine 返回事务队列
func (c *Connection) GetQueuedCmdLine() [][][]byte {
	return c.queue
}

// EnqueueCmd 执行事务时的命令入队
func (c *Connection) EnqueueCmd(cmdLine [][]byte) {
	c.queue = append(c.queue, cmdLine)
}

// ClearQueuedCmd 清除事务队列
func (c *Connection) ClearQueuedCmd() {
	c.queue = nil
}

// GetWatching 获取Watching Map
func (c *Connection) GetWatching() map[string]uint32 {
	if c.watching == nil {
		c.watching = make(map[string]uint32)
	}
	return c.watching
}

// AddTxError 添加事务执行时出现的错误
func (c *Connection) AddTxError(err error) {
	c.txErrors = append(c.txErrors, err)
}

// GetTxErrors 返回事务执行时出现的语法错误
func (c *Connection) GetTxErrors() []error {
	return c.txErrors
}

/*
 * 发布订阅相关
 */

// Subscribe 将当前连接作为给定频道的连接者
func (c *Connection) Subscribe(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.subs == nil {
		c.subs = make(map[string]bool)
	}
	c.subs[channel] = true
}

// UnSubscribe 取消将当前连接作为给定频道的连接者
func (c *Connection) UnSubscribe(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.subs) == 0 {
		return
	}
	delete(c.subs, channel)
}

// SubsCount 返回订阅频道的数量
func (c *Connection) SubsCount() int {
	return len(c.subs)
}

// GetChannels 返回所有订阅的频道
func (c *Connection) GetChannels() []string {
	if c.subs == nil {
		return make([]string, 0)
	}
	channels := make([]string, len(c.subs))
	i := 0
	for channel := range c.subs {
		channels[i] = channel
		i++
	}
	return channels
}
