package list

import (
	LinkList "container/list"
)

type LinkedList struct {
	list *LinkList.List
}

// Make 构造器
func Make(vals ...interface{}) *LinkedList {
	l := LinkedList{list: LinkList.New()}
	for _, v := range vals {
		l.list.PushBack(v)
	}
	return &l
}

// Add 添加节点到尾部
func (l *LinkedList) Add(val interface{}) {
	if l == nil {
		panic("list is nil")
	}
	l.list.PushBack(val)
}

// 私有方法，查找元素
func (l *LinkedList) find(index int) (n *LinkList.Element) {
	if index < l.Len()/2 {
		n = l.list.Front()
		for i := 0; i < index; i++ {
			n = n.Next()
		}
	} else {
		n = l.list.Back()
		for i := l.list.Len() - 1; i > index; i-- {
			n = n.Prev()
		}
	}
	return n
}

func (l *LinkedList) Get(index int) (val interface{}) {
	if l == nil {
		panic("list is nil")
	}
	if index < 0 || index >= l.list.Len() {
		panic("index out of bound")
	}
	return l.find(index).Value
}

func (l *LinkedList) Set(index int, val interface{}) {
	if l == nil {
		panic("list is nil")
	}
	if index < 0 || index >= l.Len() {
		panic("index out of bound")
	}
	n := l.find(index)
	n.Value = val
}

func (l *LinkedList) Insert(index int, val interface{}) {
	if l == nil {
		panic("list is nil")
	}
	if index < 0 || index > l.Len() {
		panic("index out of bound")
	}
	if index == l.Len() {
		l.Add(val)
		return
	}
	pivot := l.find(index)
	l.list.InsertBefore(val, pivot)
}

func (l *LinkedList) Remove(index int) (val interface{}) {
	if l == nil {
		panic("list is nil")
	}
	if index < 0 || index >= l.Len() {
		panic("index out of bound")
	}
	n := l.find(index)
	l.list.Remove(n)
	return n.Value
}

func (l *LinkedList) RemoveLast() (val interface{}) {
	if l == nil {
		panic("list is nil")
	}
	if l.list.Back() == nil {
		return nil
	}
	n := l.list.Back()
	l.list.Remove(n)
	return n.Value
}

func (l *LinkedList) RemoveAllByVal(expected Expected) int {
	if l == nil {
		panic("list is nil")
	}
	n := l.list.Front()
	removed := 0
	var nextNode *LinkList.Element
	for n != nil {
		nextNode = n.Next()
		// 用户自定义的比较方法
		if expected(n.Value) {
			l.list.Remove(n)
			removed++
		}
		n = nextNode
	}
	return removed
}

func (l *LinkedList) RemoveByVal(expected Expected, count int) int {
	if l == nil {
		panic("list is nil")
	}
	n := l.list.Front()
	removed := 0
	var nextNode *LinkList.Element
	for n != nil {
		nextNode = n.Next()
		// 用户自定义的比较方法
		if expected(n.Value) {
			l.list.Remove(n)
			removed++
		}
		if removed == count {
			break
		}
		n = nextNode
	}
	return removed
}

func (l *LinkedList) ReverseRemoveByVal(expected Expected, count int) int {
	if l == nil {
		panic("list is nil")
	}
	n := l.list.Back()
	removed := 0
	var prevNode *LinkList.Element
	for n != nil {
		prevNode = n.Prev()
		// 用户自定义的比较方法
		if expected(n.Value) {
			l.list.Remove(n)
			removed++
		}
		if removed == count {
			break
		}
		n = prevNode
	}
	return removed
}

func (l *LinkedList) Len() int {
	if l == nil {
		panic("list is nil")
	}
	return l.list.Len()
}

func (l *LinkedList) ForEach(consumer Consumer) {
	if l == nil {
		panic("list is nil")
	}
	n := l.list.Front()
	i := 0
	for n != nil {
		goNext := consumer(i, n.Value)
		if !goNext {
			break
		}
		i++
		n = n.Next()
	}
}

func (l *LinkedList) Contains(expected Expected) bool {
	contains := false
	l.ForEach(func(i int, v interface{}) bool {
		if expected(v) {
			contains = true
			return false
		}
		return true
	})
	return contains
}

func (l *LinkedList) Range(start int, stop int) (slice []interface{}) {
	if l == nil {
		panic("list is nil")
	}
	if start < 0 || start >= l.Len() {
		panic("`start` out of range")
	}
	if stop < start || stop > l.Len() {
		panic("`stop` out of range")
	}
	sliceSize := stop - start
	slice = make([]interface{}, sliceSize)
	n := l.list.Front()
	i := 0
	sliceIndex := 0
	for n != nil {
		if i >= start && i < stop {
			slice[sliceIndex] = n.Value
			sliceIndex++
		} else if i >= stop {
			break
		}
		i++
		n = n.Next()
	}
	return
}
