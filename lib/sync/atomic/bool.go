package atomic

import "sync/atomic"

// Boolean 是一个布尔类型的变量，所有的动作都是原子性的
type Boolean uint32

// Get 原子性地读取值
func (b *Boolean) Get() bool {
	return atomic.LoadUint32((*uint32)(b)) != 0
}

// Set 原子性地写入值
func (b *Boolean) Set(v bool) {
	if v {
		atomic.StoreUint32((*uint32)(b), 1)
	} else {
		atomic.StoreUint32((*uint32)(b), 0)
	}
}
