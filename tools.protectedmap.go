package main

import "github.com/heytribe/live-webrtcsignaling/my"

type ProtectedMap struct {
	my.RWMutex
	d map[string]interface{}
}

func NewProtectedMap() *ProtectedMap {
	m := new(ProtectedMap)
	m.Init()
	return m
}

func (m *ProtectedMap) Init() {
	m.d = make(map[string]interface{})
}

func (m *ProtectedMap) Set(k string, i interface{}) {
	m.Lock()
	defer m.Unlock()

	m.d[k] = i
}

func (m *ProtectedMap) Get(k string) interface{} {
	m.RLock()
	defer m.RUnlock()

	return m.d[k]
}

func (m *ProtectedMap) Del(k string) {
	m.Lock()
	defer m.Unlock()
	delete(m.d, k)
}
