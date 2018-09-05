package dtls

import (
	"unsafe"

	"github.com/heytribe/live-webrtcsignaling/my"
)

// #include <stdlib.h>
import "C"

type mapping struct {
	my.RWMutex
	values map[unsafe.Pointer]unsafe.Pointer
}

func newMapping() *mapping {
	return &mapping{
		values: make(map[unsafe.Pointer]unsafe.Pointer),
	}
}

func (m *mapping) Set(x unsafe.Pointer) unsafe.Pointer {
	res := unsafe.Pointer(C.malloc(1))

	m.Lock()
	defer m.Unlock()
	m.values[res] = x

	return res
}

func (m *mapping) Get(x unsafe.Pointer) unsafe.Pointer {
	m.RLock()
	defer m.RUnlock()
	res := m.values[x]

	return res
}

func (m *mapping) Delete(x unsafe.Pointer) {
	m.Lock()
	defer m.Unlock()
	delete(m.values, x)
	C.free(unsafe.Pointer(x))
}
