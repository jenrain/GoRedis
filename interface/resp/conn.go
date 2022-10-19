package resp

// Connection 客户端连接
type Connection interface {
	// Write 给客户端回复消息
	Write([]byte) error
	// GetDBIndex 当前用的是哪个数据库
	GetDBIndex() int
	// SelectDB 切换数据库
	SelectDB(int)
}
