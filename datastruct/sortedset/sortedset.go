package sortedset

import "strconv"

// SortedSet 有序集合结构
type SortedSet struct {
	dict     map[string]*Element
	skiplist *skiplist
}

// Make 构造器
func Make() *SortedSet {
	return &SortedSet{
		dict:     make(map[string]*Element),
		skiplist: makeSkiplist(),
	}
}

// Add 添加元素
// 返回true表示添加
// 返回false表示更新
func (sortedSet *SortedSet) Add(member string, score float64) bool {
	// 从map中找到旧元素
	element, ok := sortedSet.dict[member]
	// 直接更新map
	sortedSet.dict[member] = &Element{
		Member: member,
		Score:  score,
	}
	// 有序集合原本有该元素
	if ok {
		// 元素分数有变化
		if score != element.Score {
			sortedSet.skiplist.remove(member, element.Score)
			sortedSet.skiplist.insert(member, score)
		}
		return false
	}
	sortedSet.skiplist.insert(member, score)
	return true
}

// Len 返回有序集合的长度
func (sortedSet *SortedSet) Len() int64 {
	return int64(len(sortedSet.dict))
}

// Get 获取元素
func (sortedSet *SortedSet) Get(member string) (element *Element, ok bool) {
	element, ok = sortedSet.dict[member]
	if !ok {
		return nil, false
	}
	return element, true
}

// Remove 删除元素
func (sortedSet *SortedSet) Remove(member string) bool {
	v, ok := sortedSet.dict[member]
	if ok {
		sortedSet.skiplist.remove(member, v.Score)
		delete(sortedSet.dict, member)
		return true
	}
	return false
}

// GetRank 返回元素排名
// desc为true表示降序排序的排名
// desc为false表示升序排序的排名
func (sortedSet *SortedSet) GetRank(member string, desc bool) (rank int64) {
	element, ok := sortedSet.dict[member]
	if !ok {
		return -1
	}
	r := sortedSet.skiplist.getRank(member, element.Score)
	if desc {
		r = sortedSet.skiplist.length - r
	} else {
		r--
	}
	return r
}

// ForEach 遍历[start, stop)区间的元素（按照排名）
// desc为true表示逆序排序
// desc为false表示顺序排序
func (sortedSet *SortedSet) ForEach(start int64, stop int64, desc bool, consumer func(element *Element) bool) {
	size := int64(sortedSet.Len())
	if start < 0 || start >= size {
		panic("illegal start " + strconv.FormatInt(start, 10))
	}
	if stop < start || stop > size {
		panic("illegal end " + strconv.FormatInt(stop, 10))
	}

	// 找到开始节点
	var node *node
	if desc {
		node = sortedSet.skiplist.tail
		if start > 0 {
			node = sortedSet.skiplist.getByRank(int64(size - start))
		}
	} else {
		node = sortedSet.skiplist.header.level[0].forward
		if start > 0 {
			node = sortedSet.skiplist.getByRank(int64(start + 1))
		}
	}

	sliceSize := int(stop - start)
	for i := 0; i < sliceSize; i++ {
		if !consumer(&node.Element) {
			break
		}
		if desc {
			node = node.backward
		} else {
			node = node.level[0].forward
		}
	}
}

// Range 返回在区间[start, stop)中的元素（按照排名）
func (sortedSet *SortedSet) Range(start int64, stop int64, desc bool) []*Element {
	sliceSize := int(stop - start)
	slice := make([]*Element, sliceSize)
	i := 0
	sortedSet.ForEach(start, stop, desc, func(element *Element) bool {
		slice[i] = element
		i++
		return true
	})
	return slice
}

// Count 返回处于给定分数区间的元素个数
func (sortedSet *SortedSet) Count(min *ScoreBorder, max *ScoreBorder) int64 {
	var i int64 = 0
	// 升序遍历
	sortedSet.ForEach(0, sortedSet.Len(), false, func(element *Element) bool {
		// 是否大于区间最小值
		gtMin := min.less(element.Score)
		// 不在区间内，继续遍历
		if !gtMin {
			return true
		}
		// 是否小于最大值
		ltMax := max.greater(element.Score)
		// 不在区间内，结束遍历
		if !ltMax {
			return false
		}
		// 累加个数
		i++
		return true
	})
	return i
}

// ForEachByScore 遍历[start, stop)区间的元素（按照分数）
// desc 表示顺序访问还是逆序访问
// offset 表示偏移量
// limit个元素
func (sortedSet *SortedSet) ForEachByScore(min *ScoreBorder, max *ScoreBorder, offset int64, limit int64, desc bool, consumer func(element *Element) bool) {
	// 找到开始的节点
	var node *node
	if desc {
		node = sortedSet.skiplist.getLastInScoreRange(min, max)
	} else {
		node = sortedSet.skiplist.getFirstInScoreRange(min, max)
	}

	// 根据desc向前或者向后偏移offset个节点
	for node != nil && offset > 0 {
		if desc {
			node = node.backward
		} else {
			node = node.level[0].forward
		}
		offset--
	}

	// 开始遍历limit个元素
	for i := 0; (i < int(limit) || limit < 0) && node != nil; i++ {
		if !consumer(&node.Element) {
			break
		}
		if desc {
			node = node.backward
		} else {
			node = node.level[0].forward
		}
		if node == nil {
			break
		}
		// 判断是否超出区间
		gtMin := min.less(node.Element.Score)
		ltMax := max.greater(node.Element.Score)
		if !gtMin || !ltMax {
			break
		}
	}
}

// RangeByScore 返回处于给定分数区间内的元素
func (sortedSet *SortedSet) RangeByScore(min *ScoreBorder, max *ScoreBorder, offset int64, limit int64, desc bool) []*Element {
	if limit == 0 || offset < 0 {
		return make([]*Element, 0)
	}
	slice := make([]*Element, 0)
	sortedSet.ForEachByScore(min, max, offset, limit, desc, func(element *Element) bool {
		slice = append(slice, element)
		return true
	})
	return slice
}

// RemoveByScore 删除处于给定分数区间的元素
func (sortedSet *SortedSet) RemoveByScore(min *ScoreBorder, max *ScoreBorder) int64 {
	removed := sortedSet.skiplist.RemoveRangeByScore(min, max)
	for _, element := range removed {
		delete(sortedSet.dict, element.Member)
	}
	return int64(len(removed))
}

// RemoveByRank 删除处于[start, stop)排名区间的元素
func (sortedSet *SortedSet) RemoveByRank(start int64, stop int64) int64 {
	removed := sortedSet.skiplist.RemoveRangeByRank(start+1, stop+1)
	for _, element := range removed {
		delete(sortedSet.dict, element.Member)
	}
	return int64(len(removed))
}
