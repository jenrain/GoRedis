package resp

// Reply 对客户端的回复
type Reply interface {
	// ToBytes 转换为字节
	ToBytes() []byte
}
