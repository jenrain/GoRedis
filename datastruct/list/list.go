package list

// Expected 用于比较的自定义函数，可以被传到Contains等函数中
type Expected func(a interface{}) bool

// Consumer 用于遍历中断的函数，返回true表示继续遍历，可以在Consumer中调用Expected
type Consumer func(i int, v interface{}) bool

type List interface {
	// Add 添加节点到尾部
	Add(val interface{})
	// Get 获取节点
	Get(index int) (val interface{})
	// Set 根据下标添加节点，原位置的节点将被覆盖
	Set(index int, val interface{})
	// Insert 根据下标添加节点，后面的节点将向后移动
	Insert(index int, val interface{})
	// Remove 根据下标删除节点
	Remove(index int) (val interface{})
	// RemoveLast 删除尾节点
	RemoveLast() (val interface{})
	// RemoveAllByVal 删除给定值的所有节点
	RemoveAllByVal(expected Expected) int
	// RemoveByVal 删除给定值的count个节点，顺序
	RemoveByVal(expected Expected, count int) int
	// ReverseRemoveByVal 删除给定值的count个节点，逆序
	ReverseRemoveByVal(expected Expected, count int) int
	// Len 获取列表长度
	Len() int
	// ForEach 遍历列表
	ForEach(consumer Consumer)
	// Contains 查看列表中是否有包含的值
	Contains(expected Expected) bool
	// Range 返回索引在 [start, stop) 内的元素
	Range(start int, stop int) []interface{}
}
