package sortedset

import "math/rand"

const (
	maxLevel = 16
)

// Element 是一个key-score对组
type Element struct {
	Member string
	// 跳跃表节点依照Score升序排序，若一样，则按照Member的字典升序排序
	Score float64
}

// Level 层
type Level struct {
	// 指向前面一个节点
	forward *node
	// 与前一个节点的跨度
	span int64
}

// node 跳跃表的一个节点
type node struct {
	Element
	// 回退指针
	backward *node
	// 每个节点有 1~maxLevel 个层级
	level []*Level
}

// skiplist 跳表结构
type skiplist struct {
	// 指向表头节点
	header *node
	// 指向表尾节点
	tail *node
	// 跳跃表的长度（除了第一个节点）
	length int64
	// 跳跃表的最大层级（除了第一个节点）
	level int16
}

// makeNode 创建一个跳跃表节点
func makeNode(level int16, score float64, member string) *node {
	n := &node{
		Element: Element{
			Score:  score,
			Member: member,
		},
		level: make([]*Level, level),
	}
	for i := range n.level {
		n.level[i] = new(Level)
	}
	return n
}

// makeSkiplist 创建一个跳跃表结构
func makeSkiplist() *skiplist {
	return &skiplist{
		level:  1,
		header: makeNode(maxLevel, 0, ""),
	}
}

// randomLevel 随机生成一个新跳跃表节点的层数（1~32）
// 满足幂次定律
func randomLevel() int16 {
	level := int16(1)
	for float32(rand.Int31()&0xFFFF) < (0.25 * 0xFFFF) {
		level++
	}
	if level < maxLevel {
		return level
	}
	return maxLevel
}

// insert 插入元素
func (skiplist *skiplist) insert(member string, score float64) *node {
	// 保存在每一层，待插入节点的前一个节点
	update := make([]*node, maxLevel)
	// 用于累加跨度
	rank := make([]int64, maxLevel)

	// 找到待插入的位置
	node := skiplist.header
	for i := skiplist.level - 1; i >= 0; i-- {
		if i == skiplist.level-1 {
			rank[i] = 0
		} else {
			// 累加跨度
			rank[i] = rank[i+1]
		}
		if node.level[i] != nil {
			// 在第i层找待插入的位置
			for node.level[i].forward != nil &&
				(node.level[i].forward.Score < score ||
					(node.level[i].forward.Score == score && node.level[i].forward.Member < member)) { // same score, different key
				// 累加与前一个节点的跨度
				rank[i] += node.level[i].span
				// 前进
				node = node.level[i].forward
			}
		}
		update[i] = node
	}
	level := randomLevel()

	// 如果新插入的节点抽到的层级最大
	if level > skiplist.level {
		// 初始化每一层的状态
		for i := skiplist.level; i < level; i++ {
			rank[i] = 0
			update[i] = skiplist.header
			update[i].level[i].span = skiplist.length
		}
		skiplist.level = level
	}

	// 构造新节点并插入到跳表
	node = makeNode(level, score, member)
	for i := int16(0); i < level; i++ {
		node.level[i].forward = update[i].level[i].forward
		update[i].level[i].forward = node

		node.level[i].span = update[i].level[i].span - (rank[0] - rank[i])
		update[i].level[i].span = (rank[0] - rank[i]) + 1
	}

	// 新插入的节点增加了前面节点的跨度
	for i := level; i < skiplist.level; i++ {
		update[i].level[i].span++
	}

	// 设置回退节点
	if update[0] == skiplist.header {
		node.backward = nil
	} else {
		node.backward = update[0]
	}
	// 设置node前面一个节点的回退节点
	if node.level[0].forward != nil {
		node.level[0].forward.backward = node
	}
	skiplist.length++
	return node
}

// 删除找到的节点
func (skiplist *skiplist) removeNode(node *node, update []*node) {
	// 更新每一层的状态
	for i := int16(0); i < skiplist.level; i++ {
		if update[i].level[i].forward == node {
			update[i].level[i].span += node.level[i].span - 1
			update[i].level[i].forward = node.level[i].forward
		} else {
			update[i].level[i].span--
		}
	}
	// 更新后面一个节点的回退指针
	if node.level[0].forward != nil {
		node.level[0].forward.backward = node.backward
	} else {
		skiplist.tail = node.backward
	}
	// 更新跳表中的最大层级
	for skiplist.level > 1 && skiplist.header.level[skiplist.level-1].forward == nil {
		skiplist.level--
	}
	skiplist.length--
}

// 寻找待删除的节点
func (skiplist *skiplist) remove(member string, score float64) bool {
	// 储存待删除节点每一层的上一个节点
	update := make([]*node, maxLevel)
	node := skiplist.header
	// 寻找待删除节点
	for i := skiplist.level - 1; i >= 0; i-- {
		for node.level[i].forward != nil &&
			(node.level[i].forward.Score < score ||
				(node.level[i].forward.Score == score &&
					node.level[i].forward.Member < member)) {
			node = node.level[i].forward
		}
		update[i] = node
	}
	// node在循环中，一直是待删除节点的前一个节点
	// 在最底层的索引处向后移动一位，刚好就是待删除节点
	node = node.level[0].forward
	// 找到该节点
	if node != nil && score == node.Score && node.Member == member {
		skiplist.removeNode(node, update)
		return true
	}
	return false
}

// 获取元素的排名，0表示没有找到元素
func (skiplist *skiplist) getRank(member string, score float64) int64 {
	var rank int64 = 0
	x := skiplist.header
	for i := skiplist.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil &&
			(x.level[i].forward.Score < score ||
				(x.level[i].forward.Score == score &&
					x.level[i].forward.Member <= member)) {
			rank += x.level[i].span
			x = x.level[i].forward
		}

		if x.Member == member {
			return rank
		}
	}
	return 0
}

// 通过排名获取元素
func (skiplist *skiplist) getByRank(rank int64) *node {
	// 记录从头节点开始的跨度
	var i int64 = 0
	// 用于遍历节点的指针
	n := skiplist.header

	// 从最高层级开始遍历
	for level := skiplist.level - 1; level >= 0; level-- {
		for n.level[level].forward != nil && (i+n.level[level].span) <= rank {
			i += n.level[level].span
			n = n.level[level].forward
		}
		if i == rank {
			return n
		}
	}
	return nil
}

// 判断[min, max]区间与是否在skiplist的分数区间内（是否有重合）
func (skiplist *skiplist) hasInRange(min *ScoreBorder, max *ScoreBorder) bool {
	// [min, max]无意义或为空
	if min.Value > max.Value || (min.Value == max.Value && (min.Exclude || max.Exclude)) {
		return false
	}
	// [min, max] > skiplist.tail.Score
	n := skiplist.tail
	if n == nil || !min.less(n.Score) {
		return false
	}
	// [min, max] < skiplist.head.Score
	n = skiplist.header.level[0].forward
	if n == nil || !max.greater(n.Score) {
		return false
	}
	return true
}

// 从跳表中找到处于[min, max]区间的最小值
func (skiplist *skiplist) getFirstInScoreRange(min *ScoreBorder, max *ScoreBorder) *node {
	if !skiplist.hasInRange(min, max) {
		return nil
	}

	n := skiplist.header
	// 找到第一个大于等于min的节点
	for level := skiplist.level - 1; level >= 0; level-- {
		for n.level[level].forward != nil && !min.less(n.level[level].forward.Score) {
			n = n.level[level].forward
		}
	}
	n = n.level[0].forward

	// n节点的分数在[min, max]区间之外
	if !max.greater(n.Score) {
		return nil
	}
	return n
}

// 从跳表中找到处于[min, max]区间的最大值
func (skiplist *skiplist) getLastInScoreRange(min *ScoreBorder, max *ScoreBorder) *node {
	if !skiplist.hasInRange(min, max) {
		return nil
	}
	n := skiplist.header
	// 找到第一个大于max的节点 的前一个节点
	for level := skiplist.level - 1; level >= 0; level-- {
		for n.level[level].forward != nil && max.greater(n.level[level].forward.Score) {
			n = n.level[level].forward
		}
	}
	// n节点的分数在[min, max]区间之外
	if !min.less(n.Score) {
		return nil
	}
	return n
}

// RemoveRangeByScore 删除跳表中分数值处在[min, max]区间内的元素，并返回它们的切片
func (skiplist *skiplist) RemoveRangeByScore(min *ScoreBorder, max *ScoreBorder) (removed []*Element) {
	// 储存待删除节点每一层的前驱节点
	update := make([]*node, maxLevel)
	removed = make([]*Element, 0)
	// 找到待删除节点每一层的前驱节点
	node := skiplist.header
	for i := skiplist.level - 1; i >= 0; i-- {
		for node.level[i].forward != nil {
			if min.less(node.level[i].forward.Score) {
				break
			}
			node = node.level[i].forward
		}
		update[i] = node
	}

	node = node.level[0].forward

	// 开始删除节点
	for node != nil {
		// 保证不超出[min, max]区间
		if !max.greater(node.Score) {
			break
		}
		next := node.level[0].forward
		removedElement := node.Element
		removed = append(removed, &removedElement)
		skiplist.removeNode(node, update)
		node = next
	}
	return removed
}

// RemoveRangeByRank 删除排名在[start, stop]区间内的元素，并返回它们的切片
func (skiplist *skiplist) RemoveRangeByRank(start int64, stop int64) (removed []*Element) {
	// 排名迭代器
	var i int64 = 0
	update := make([]*node, maxLevel)
	removed = make([]*Element, 0)

	// 找到待删除的第一个节点的前驱节点，并储存在update切片中
	node := skiplist.header
	for level := skiplist.level - 1; level >= 0; level-- {
		for node.level[level].forward != nil && (i+node.level[level].span) < start {
			i += node.level[level].span
			node = node.level[level].forward
		}
		update[level] = node
	}

	i++
	// 处在区间的第一个节点
	node = node.level[0].forward

	// 开始删除节点
	for node != nil && i < stop {
		next := node.level[0].forward
		removedElement := node.Element
		removed = append(removed, &removedElement)
		skiplist.removeNode(node, update)
		node = next
		i++
	}
	return removed
}
