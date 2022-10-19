package tcp

import (
	"GoRedis/interface/tcp"
	"GoRedis/lib/logger"
	"context"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Config 用于tcp连接的配置
type Config struct {
	Address string
}

// ListenAndServeWithSignal 该函数的主要功能是绑定端口并处理请求、监听是否有来自系统的关闭信号
func ListenAndServeWithSignal(cfg *Config, handler tcp.Handler) error {
	closeChan := make(chan struct{})
	sigChan := make(chan os.Signal)
	// 将系统信号转发给sigChan
	// Notify函数让signal包将输入信号转发到c，如果没有列出要传递的信号，会将所有输入信号传递到c，否则只传递列出的输入信号
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	// 如果收到系统发送的关闭信号，就向channel中发送空结构体，下面的方法就会立刻关闭连接
	// 如果未收到系统发送的关闭信号，协程将阻塞
	go func() {
		sig := <-sigChan
		switch sig {
		case syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			closeChan <- struct{}{}
		}
	}()

	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return err
	}
	logger.Info("start listen")
	ListenAndServe(listener, handler, closeChan)
	return nil
}

// ListenAndServe 处理客户端连接
func ListenAndServe(listener net.Listener, handler tcp.Handler, closeChan <-chan struct{}) {
	go func() {
		// 收到系统的关闭信号，马上关闭连接
		<-closeChan
		logger.Info("shutting down")
		_ = listener.Close()
		_ = handler.Close()
	}()
	defer func() {
		_ = listener.Close()
		_ = handler.Close()
	}()
	// 创建一个空的上下文
	ctx := context.Background()
	// 如果新连接出错，需要等待所有已经连接的客户端退出再退出
	var waitDone sync.WaitGroup
	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		logger.Info("accepted link")
		// 每处理一个客户端业务，就向等待队列+1
		waitDone.Add(1)
		go func() {
			defer func() {
				waitDone.Done()
			}()
			// 业务逻辑
			handler.Handle(ctx, conn)
		}()

	}
	waitDone.Wait()
}
