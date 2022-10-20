package dict

// SimpleDict 封装了一个map，她不是线程安全的
type SimpleDict struct {
	m map[string]interface{}
}

// MakeSimpleDict 创建新map
func MakeSimpleDict() *SimpleDict {
	return &SimpleDict{
		m: make(map[string]interface{}),
	}
}

// Get 返回key对应的value，以及判断key是否存在
func (dict *SimpleDict) Get(key string) (val interface{}, exists bool) {
	val, ok := dict.m[key]
	return val, ok
}

// Len 返回map的元素个数
func (dict *SimpleDict) Len() int {
	if dict.m == nil {
		panic("m is nil")
	}
	return len(dict.m)
}

// Put 向map中插入k-v，并返回插入成功的k-v个数
func (dict *SimpleDict) Put(key string, val interface{}) (result int) {
	_, existed := dict.m[key]
	dict.m[key] = val
	if existed {
		return 0
	}
	return 1
}

// PutIfAbsent 如果key不存在，就向map中插入k-v，并返回插入成功的k-v个数
func (dict *SimpleDict) PutIfAbsent(key string, val interface{}) (result int) {
	_, existed := dict.m[key]
	if existed {
		return 0
	}
	dict.m[key] = val
	return 1
}

// PutIfExist 如果key存在，就向map中插入k-v，并返回插入成功的k-v个数
func (dict *SimpleDict) PutIfExist(key string, val interface{}) (result int) {
	_, existed := dict.m[key]
	if existed {
		dict.m[key] = val
		return 1
	}
	return 0
}

// Remove 删除键并返回已删除键值的数目
func (dict *SimpleDict) Remove(key string) (result int) {
	_, existed := dict.m[key]
	delete(dict.m, key)
	if existed {
		return 1
	}
	return 0
}

// Keys 以切片的形式返回map中的所有元素
func (dict *SimpleDict) Keys() []string {
	result := make([]string, len(dict.m))
	i := 0
	for k := range dict.m {
		result[i] = k
		i++
	}
	return result
}

// ForEach 遍历map
func (dict *SimpleDict) ForEach(consumer Consumer) {
	for k, v := range dict.m {
		if !consumer(k, v) {
			break
		}
	}
}

// RandomKeys 随机返回limit个key，key可能会重复
func (dict *SimpleDict) RandomKeys(limit int) []string {
	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		for k := range dict.m {
			result[i] = k
			break
		}
	}
	return result
}

// RandomDistinctKeys 随机返回limit个key，key不会重复
func (dict *SimpleDict) RandomDistinctKeys(limit int) []string {
	size := limit
	if size > len(dict.m) {
		size = len(dict.m)
	}
	result := make([]string, size)
	i := 0
	for k := range dict.m {
		if i == size {
			break
		}
		result[i] = k
		i++
	}
	return result
}

// Clear 删除map中的所有key
func (dict *SimpleDict) Clear() {
	*dict = *MakeSimpleDict()
}
