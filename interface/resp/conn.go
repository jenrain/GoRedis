package resp

// Connection 客户端连接
type Connection interface {
	// Write 给客户端回复消息
	Write([]byte) error
	// GetDBIndex 当前用的是哪个数据库
	GetDBIndex() int
	// SelectDB 切换数据库
	SelectDB(int)

	/*
	 *	事务相关
	 */
	// InMultiState 判断是否正在执行事务
	InMultiState() bool
	// SetMultiState 设置事务的执行状态
	SetMultiState(bool)
	// GetQueuedCmdLine 获取事务队列
	GetQueuedCmdLine() [][][]byte
	// EnqueueCmd 将事务命令入队
	EnqueueCmd([][]byte)
	// ClearQueuedCmd 清除事务队列中的命令
	ClearQueuedCmd()
	// GetWatching 获取Watching Map
	GetWatching() map[string]uint32
	// AddTxError 添加事务执行时出现的错误
	AddTxError(err error)
	// GetTxErrors 返回事务执行时出现的语法错误
	GetTxErrors() []error

	/*
	 * 发布订阅相关
	 */
	// Subscribe 订阅频道
	Subscribe(channel string)
	// UnSubscribe 退订频道
	UnSubscribe(channel string)
	// SubsCount 返回频道的订阅者数量
	SubsCount() int
	// GetChannels 获取所有频道
	GetChannels() []string
}
