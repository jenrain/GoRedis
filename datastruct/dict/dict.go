package dict

// Consumer 用于遍历map，返回true就继续往后遍历
type Consumer func(key string, val interface{}) bool

type Dict interface {
	Get(key string) (val interface{}, exist bool)
	Len() int
	Put(key string, val interface{}) (result int)
	// PutIfAbsent 如果不存在就往map添加值
	PutIfAbsent(key string, val interface{}) (result int)
	// PutIfExist 修改值
	PutIfExist(key string, val interface{}) (result int)
	Remove(key string) (result int)
	// ForEach 遍历字典
	ForEach(consumer Consumer)
	// Keys 列出所有的键
	Keys() []string
	// RandomKeys 随机列出指定数量的键（可以重复）
	RandomKeys(limit int) []string
	// RandomDistinctKeys 随机列出指定数量的键（无重复）
	RandomDistinctKeys(limit int) []string
	// Clear 清空字典
	Clear()
}
