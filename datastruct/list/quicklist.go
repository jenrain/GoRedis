package list

import "container/list"

// pageSize 一页的大小
const pageSize = 1024

// QuickList 快速列表是在页（切片）之上建立的链表
// QuickList 比普通链表的插入、遍历以及内存有着更好的性能
type QuickList struct {
	data *list.List // 每一页就是interface{}的切片，大小为1024
	size int
}

// iterator 快速列表的迭代器，在[-1, ql.Len()]之间迭代
type iterator struct {
	// 快速列表的一页
	node *list.Element
	// 元素下标在页中的偏移量
	offset int
	ql     *QuickList
}

func NewQuickList() *QuickList {
	l := &QuickList{
		data: list.New(),
	}
	return l
}

// Add 添加元素到表尾
func (ql *QuickList) Add(val interface{}) {
	ql.size++
	// 列表是空的
	if ql.data.Len() == 0 {
		page := make([]interface{}, 0, pageSize)
		page = append(page, val)
		ql.data.PushBack(page)
		return
	}
	// 获取表尾节点
	backNode := ql.data.Back()
	backPage := backNode.Value.([]interface{})
	// 表尾节点页满了，需要新创建一页
	if len(backPage) == cap(backPage) {
		page := make([]interface{}, 0, pageSize)
		page = append(page, val)
		ql.data.PushBack(page)
		return
	}
	// 默认将节点添加进表尾页中
	backPage = append(backPage, val)
	backNode.Value = backPage
}

// find 根据元素下标返回对应的迭代器
func (ql *QuickList) find(index int) *iterator {
	if ql == nil {
		panic("list is nil")
	}
	if index < 0 || index >= ql.size {
		panic("index out of bound")
	}
	var n *list.Element
	var page []interface{}
	var pageBeg int
	if index < ql.size/2 {
		// 从表头进行查找
		n = ql.data.Front()
		pageBeg = 0
		for {
			page = n.Value.([]interface{})
			if pageBeg+len(page) > index {
				break
			}
			pageBeg += len(page)
			n = n.Next()
		}
	} else {
		// 从表尾进行查找
		n = ql.data.Back()
		pageBeg = ql.size
		for {
			page = n.Value.([]interface{})
			pageBeg -= len(page)
			if pageBeg <= index {
				break
			}
			n = n.Prev()
		}
	}
	pageOffset := index - pageBeg
	return &iterator{
		node:   n,
		offset: pageOffset,
		ql:     ql,
	}
}

// 使用迭代器返回一个元素
func (iter *iterator) get() interface{} {
	return iter.page()[iter.offset]
}

// 返回迭代器对应的那一页
func (iter *iterator) page() []interface{} {
	return iter.node.Value.([]interface{})
}

// next 页内迭代器，向后迭代一位
// 如果当前元素下标未出界且不在最后一位，就向后移动一位，返回true
// 如果当前元素下标在快速列表的最后一页且是最后一个元素，直接返回false
// 如果当前元素下标不在快速列表的最后一页，但是是当前页的最后一个元素，跳转到下一页，返回true
func (iter *iterator) next() bool {
	// 得到迭代器对应的那一页
	page := iter.page()
	// 当前位置未出界且不在最后一位，就向后移动一位，返回true
	if iter.offset < len(page)-1 {
		iter.offset++
		return true
	}
	// 当前元素在快速列表的最后一页且是最后一个元素，直接返回false
	if iter.node == iter.ql.data.Back() {
		// already at last node
		iter.offset = len(page)
		return false
	}
	// 当前元素不在快速列表的最后一页，但是是当前页的最后一个元素，跳转到下一页，返回true
	iter.offset = 0
	iter.node = iter.node.Next()
	return true
}

// prev 页内迭代器，向前迭代一位
func (iter *iterator) prev() bool {
	if iter.offset > 0 {
		iter.offset--
		return true
	}
	if iter.node == iter.ql.data.Front() {
		iter.offset = -1
		return false
	}
	iter.node = iter.node.Prev()
	prevPage := iter.node.Value.([]interface{})
	iter.offset = len(prevPage) - 1
	return true
}

// 判断是否到达最后一页的最后一个元素
func (iter *iterator) atEnd() bool {
	if iter.ql.data.Len() == 0 {
		return true
	}
	if iter.node != iter.ql.data.Back() {
		return false
	}
	page := iter.page()
	return iter.offset == len(page)
}

// 判断是否是第一页的第一个元素
func (iter *iterator) atBegin() bool {
	if iter.ql.data.Len() == 0 {
		return true
	}
	if iter.node != iter.ql.data.Front() {
		return false
	}
	return iter.offset == -1
}

// Get 返回给定下标的元素
func (ql *QuickList) Get(index int) (val interface{}) {
	iter := ql.find(index)
	return iter.get()
}

// 设置迭代器指向的元素为val
func (iter *iterator) set(val interface{}) {
	page := iter.page()
	page[iter.offset] = val
}

// Set 根据下标更新元素
func (ql *QuickList) Set(index int, val interface{}) {
	iter := ql.find(index)
	iter.set(val)
}

// Insert 插入元素
// 插入元素的策略分三种情况：
// 1. 向最后一页的最后一个位置插入元素，直接掉用ql.Add()插入即可
// 2. 某一页插入一个元素，且该页未满，直接插入该页即可
// 3. 某一页插入一个元素，该页满了，就新创建一页，然后将前512个元素留在原来那页，将后512个元素移到新的页中，
//    新插入的元素，如果下标在[0,512]之间，就插入到原来页，如果下标在[516, 1024]之间，就插入到新创建的页中
func (ql *QuickList) Insert(index int, val interface{}) {
	// 向表尾插入元素
	if index == ql.size {
		ql.Add(val)
		return
	}
	iter := ql.find(index)
	page := iter.node.Value.([]interface{})
	// 如果待插入页的元素小于1024，直接插入到该页即可
	if len(page) < pageSize {
		// insert into not full page
		page = append(page[:iter.offset+1], page[iter.offset:]...)
		page[iter.offset] = val
		iter.node.Value = page
		ql.size++
		return
	}
	// 待插入页的元素已经满1024，就需要新创建一页
	var nextPage []interface{}
	// 后一半的元素放在新创建的页中，前一半元素放在原来的页中
	nextPage = append(nextPage, page[pageSize/2:]...) // pageSize must be even
	page = page[:pageSize/2]
	// 待插入元素的下标小于512，插到前面那页
	if iter.offset < len(page) {
		page = append(page[:iter.offset+1], page[iter.offset:]...)
		page[iter.offset] = val
	} else {
		// 待插入元素的下标大于512，插到后面那页
		i := iter.offset - pageSize/2
		nextPage = append(nextPage[:i+1], nextPage[i:]...)
		nextPage[i] = val
	}
	// 储存当前页和新创建的下一页
	iter.node.Value = page
	ql.data.InsertAfter(nextPage, iter.node)
	ql.size++
}

// 删除元素
// 删除元素分为三种情况：
// 1.删除后的页不为空，且删除的不是该页的最后一个元素，什么都不用管
// 2.删除后的页不为空，且删除的是该页的最后一个元素，需要将迭代器移动到下一页的最后一个元素
// 3.删除的页为空（需要删除该页），且删除的页是最后一页，将迭代器置空
// 4.删除的页为空（需要删除该页），且删除的页不是最后一页，将迭代器指向下一页
func (iter *iterator) remove() interface{} {
	page := iter.page()
	val := page[iter.offset]
	// 先直接在页中删除这个元素
	page = append(page[:iter.offset], page[iter.offset+1:]...)
	// 如果删除后的页不为空，只更新iter.offset即可
	if len(page) > 0 {
		iter.node.Value = page
		// 如果删除的是页中的最后一个元素，那么迭代器需要移动到下一页的第一个元素
		if iter.offset == len(page) {
			if iter.node != iter.ql.data.Back() {
				iter.node = iter.node.Next()
				iter.offset = 0
			}
		}
	} else {
		// 如果删除后的页为空，需要删除该页
		// 如果删除的是最后一页，迭代器需要置空
		if iter.node == iter.ql.data.Back() {
			iter.ql.data.Remove(iter.node)
			iter.node = nil
			iter.offset = 0
		} else {
			// 如果删除的不是最后一页，迭代器需要指向下一页
			nextNode := iter.node.Next()
			iter.ql.data.Remove(iter.node)
			iter.node = nextNode
			iter.offset = 0
		}
	}
	iter.ql.size--
	return val
}

// Remove 删除给定下标的元素
func (ql *QuickList) Remove(index int) interface{} {
	iter := ql.find(index)
	return iter.remove()
}

// Len 返回快速列表的元素个数
func (ql *QuickList) Len() int {
	return ql.size
}

// RemoveLast 删除最后一个元素并返回其值
func (ql *QuickList) RemoveLast() interface{} {
	if ql.Len() == 0 {
		return nil
	}
	ql.size--
	lastNode := ql.data.Back()
	lastPage := lastNode.Value.([]interface{})
	if len(lastPage) == 1 {
		ql.data.Remove(lastNode)
		return lastPage[0]
	}
	val := lastPage[len(lastPage)-1]
	lastPage = lastPage[:len(lastPage)-1]
	lastNode.Value = lastPage
	return val
}

// RemoveAllByVal 删除符合条件的所有元素，并返回删除的个数
// 传入一个函数，将值作为形参传入该函数，如果返回true就删除
func (ql *QuickList) RemoveAllByVal(expected Expected) int {
	iter := ql.find(0)
	removed := 0
	for !iter.atEnd() {
		if expected(iter.get()) {
			iter.remove()
			removed++
		} else {
			iter.next()
		}
	}
	return removed
}

// RemoveByVal 删除给定值的count个元素，顺序
func (ql *QuickList) RemoveByVal(expected Expected, count int) int {
	if ql.size == 0 {
		return 0
	}
	iter := ql.find(0)
	removed := 0
	for !iter.atEnd() {
		if expected(iter.get()) {
			iter.remove()
			removed++
			if removed == count {
				break
			}
		} else {
			iter.next()
		}
	}
	return removed
}

// ReverseRemoveByVal 删除给定值的count个元素，逆序
func (ql *QuickList) ReverseRemoveByVal(expected Expected, count int) int {
	if ql.size == 0 {
		return 0
	}
	iter := ql.find(ql.size - 1)
	removed := 0
	for !iter.atBegin() {
		if expected(iter.get()) {
			iter.remove()
			removed++
			if removed == count {
				break
			}
		}
		iter.prev()
	}
	return removed
}

// ForEach 遍历快速列表中的元素
// 如果consumer返回false，结束遍历
func (ql *QuickList) ForEach(consumer Consumer) {
	if ql == nil {
		panic("list is nil")
	}
	if ql.Len() == 0 {
		return
	}
	iter := ql.find(0)
	i := 0
	for {
		goNext := consumer(i, iter.get())
		if !goNext {
			break
		}
		i++
		// 遍历到表尾，结束
		if !iter.next() {
			break
		}
	}
}

// Contains 查看列表中是否有包含的值
func (ql *QuickList) Contains(expected Expected) bool {
	contains := false
	ql.ForEach(func(i int, actual interface{}) bool {
		// 有包含的值，结束遍历
		if expected(actual) {
			contains = true
			return false
		}
		return true
	})
	return contains
}

// Range 返回下标在 [start, stop) 之间的元素
func (ql *QuickList) Range(start int, stop int) []interface{} {
	if start < 0 || start >= ql.Len() {
		panic("`start` out of range")
	}
	if stop < start || stop > ql.Len() {
		panic("`stop` out of range")
	}
	sliceSize := stop - start
	slice := make([]interface{}, 0, sliceSize)
	iter := ql.find(start)
	i := 0
	for i < sliceSize {
		slice = append(slice, iter.get())
		iter.next()
		i++
	}
	return slice
}
