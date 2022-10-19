package set

import (
	"GoRedis/datastruct/dict"
)

// Set 集合
type Set struct {
	dict dict.Dict
}

// Make 构造器
func Make(members ...string) *Set {
	set := &Set{dict: dict.MakeSyncDict()}
	for _, member := range members {
		set.Add(member)
	}
	return set
}

// Add 添加元素
func (set *Set) Add(val string) int {
	return set.dict.Put(val, struct{}{})
}

// Remove 删除元素
func (set *Set) Remove(val string) int {
	return set.dict.Remove(val)
}

// Has 判断元素是否存在
func (set *Set) Has(val string) bool {
	_, exists := set.dict.Get(val)
	return exists
}

// Len 返回元素长度
func (set *Set) Len() int {
	return set.dict.Len()
}

// ToSlice 以切片形式返回集合所有元素
func (set *Set) ToSlice() []string {
	slice := make([]string, set.Len())
	i := 0
	set.dict.ForEach(func(key string, val interface{}) bool {
		if i < len(slice) {
			slice[i] = key
		} else {
			slice = append(slice, key)
		}
		i++
		return true
	})
	return slice
}

// ForEach 遍历集合
func (set *Set) ForEach(consumer func(member string) bool) {
	set.dict.ForEach(func(key string, val interface{}) bool {
		return consumer(key)
	})
}

// Intersect 交集
func (set *Set) Intersect(another *Set) *Set {
	if set == nil {
		panic("set is nil")
	}
	result := Make()
	another.ForEach(func(member string) bool {
		if member != "" {
			if set.Has(member) {
				result.Add(member)
			}
		}
		return true
	})
	return result
}

// Union 并集
func (set *Set) Union(another *Set) *Set {
	if set == nil {
		panic("set is nil")
	}
	result := Make()
	set.ForEach(func(member string) bool {
		if member != "" {
			result.Add(member)
		}
		return true
	})
	another.ForEach(func(member string) bool {
		if member != "" {
			result.Add(member)
		}
		return true
	})
	return result
}

// Diff 差集
func (set *Set) Diff(another *Set) *Set {
	if set == nil {
		panic("set is nil")
	}
	result := Make()
	set.ForEach(func(member string) bool {
		if member != "" {
			if !another.Has(member) {
				result.Add(member)
			}
		}
		return true
	})
	return result
}

// RandomMembers 随机返回limit个元素
func (set *Set) RandomMembers(limit int) []string {
	return set.dict.RandomKeys(limit)
}

// RandomDistinctMembers 不重复的随机返回limit个元素
func (set *Set) RandomDistinctMembers(limit int) []string {
	return set.dict.RandomDistinctKeys(limit)
}
