package dict

import (
	"sync"
)

type SyncDict struct {
	m sync.Map
}

func MakeSyncDict() *SyncDict {
	return &SyncDict{}
}

func (s *SyncDict) Get(key string) (val interface{}, exist bool) {
	val, ok := s.m.Load(key)
	return val, ok
}

func (s *SyncDict) Len() int {
	length := 0
	s.m.Range(func(key, value interface{}) bool {
		length++
		return true
	})
	return length
}

func (s *SyncDict) Put(key string, val interface{}) (result int) {
	_, existed := s.m.Load(key)
	s.m.Store(key, val)
	// 修改
	if existed {
		return 0
	}
	// 插入新值
	return 1
}

// PutIfAbsent 如果不存在就往map添加值
func (s *SyncDict) PutIfAbsent(key string, val interface{}) (result int) {
	_, existed := s.m.Load(key)
	if existed {
		return 0
	}
	s.m.Store(key, val)
	return 1
}

// PutIfExist 修改值
func (s *SyncDict) PutIfExist(key string, val interface{}) (result int) {
	_, existed := s.m.Load(key)
	if existed {
		s.m.Store(key, val)
		return 1
	}
	return 0
}

func (s *SyncDict) Remove(key string) (result int) {
	_, existed := s.m.Load(key)
	if existed {
		s.m.Delete(key)
		return 1
	}
	return 0
}

func (s *SyncDict) ForEach(consumer Consumer) {
	s.m.Range(func(key, value interface{}) bool {
		consumer(key.(string), value)
		return true
	})
}

func (s *SyncDict) Keys() []string {
	result := make([]string, s.Len())
	i := 0
	s.m.Range(func(key, value interface{}) bool {
		result[i] = key.(string)
		i++
		return true
	})
	return result
}

func (s *SyncDict) RandomKeys(limit int) []string {
	result := make([]string, s.Len())
	for i := 0; i < limit; i++ {
		// 随机取出一个值
		s.m.Range(func(key, value interface{}) bool {
			result[i] = key.(string)
			return false
		})
	}
	return result
}

func (s *SyncDict) RandomDistinctKeys(limit int) []string {
	result := make([]string, s.Len())
	i := 0
	s.m.Range(func(key, value interface{}) bool {
		result[i] = key.(string)
		i++
		if i == limit {
			return false
		}
		return true
	})
	return result
}

func (s *SyncDict) Clear() {
	*s = *MakeSyncDict()
}
