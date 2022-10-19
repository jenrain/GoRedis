package tcp

import (
	"context"
	"net"
)

// Handler 将处理连接的操作进行封装
type Handler interface {
	Handle(ctx context.Context, conn net.Conn)
	Close() error
}
