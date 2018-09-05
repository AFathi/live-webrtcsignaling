package my

/*
 * providing drop-in replacement for sync.Mutex & sync.RWMutex
 * in "development" env, using go-deadlock
 *
 * provide PLMutex & PLRWMutex for contextual plogger mutex
 *
 * also, provide NamedMutex & NamedRWMutex for higher level mutex
 */

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	plogger "github.com/heytribe/go-plogger"
	"github.com/sasha-s/go-deadlock"
)

// shouldn't race (init only)
var deadlockDetection = false

// number of locks
var locknum int64

func EnableDeadlockDetection() {
	deadlockDetection = true
}

/*
 * RWMutex is a drop-in RWMutex replacement
 *  with alternate deadlock detection.
 *
 * fixme: check memory footprint
 */
type RWMutex struct {
	sync.RWMutex
	alt deadlock.RWMutex // alternate debug mutex
}

func (o *RWMutex) Lock() {
	if deadlockDetection {
		atomic.AddInt64(&locknum, 1)
		o.alt.Lock()
	} else {
		o.RWMutex.Lock()
	}
}

func (o *RWMutex) Unlock() {
	if deadlockDetection {
		o.alt.Unlock()
		atomic.AddInt64(&locknum, -1)
	} else {
		o.RWMutex.Unlock()
	}
}

func (o *RWMutex) RLock() {
	if deadlockDetection {
		atomic.AddInt64(&locknum, 1)
		o.alt.RLock()
	} else {
		o.RWMutex.RLock()
	}
}

func (o *RWMutex) RUnlock() {
	if deadlockDetection {
		o.alt.RUnlock()
		atomic.AddInt64(&locknum, -1)
	} else {
		o.RWMutex.RUnlock()
	}
}

/*
 * Mutex is a drop-in Mutex replacement
 *  with alternate deadlock detection.
 */
type Mutex struct {
	sync.Mutex
	alt deadlock.Mutex // alternate debug mutex
}

func (o *Mutex) Lock() {
	if deadlockDetection {
		atomic.AddInt64(&locknum, 1)
		o.alt.Lock()
	} else {
		o.Mutex.Lock()
	}
}

func (o *Mutex) Unlock() {
	if deadlockDetection {
		o.alt.Unlock()
		atomic.AddInt64(&locknum, -1)
	} else {
		o.Mutex.Unlock()
	}
}

/*
 * PLRWMutex is a wrapper around RWMutex
 */
type PLRWMutex struct {
	RWMutex
}

func (o *PLRWMutex) Lock(ctx context.Context, format string, args ...interface{}) {
	log := plogger.FromContextSafe(ctx).Prefix("PLRWMutex").Tag("mutex")
	s := ""
	if deadlockDetection {
		s = " - using deadlock detection"
		s += fmt.Sprintf(" (%d->%d)", locknum, locknum+1)
	}
	log.Debugf("["+format+"] Lock"+s, args...)
	o.RWMutex.Lock()
	log.Debugf("["+format+"] Lock OK", args...)
}

func (o *PLRWMutex) Unlock(ctx context.Context, format string, args ...interface{}) {
	log := plogger.FromContextSafe(ctx).Prefix("PLRWMutex").Tag("mutex")
	s := ""
	if deadlockDetection {
		s = " - using deadlock detection"
		s += fmt.Sprintf(" (%d->%d)", locknum, locknum-1)
	}
	log.Debugf("["+format+"] Unlock"+s, args...)
	o.RWMutex.Unlock()
	log.Debugf("["+format+"] Unlock OK", args...)
}

func (o *PLRWMutex) RLock(ctx context.Context, format string, args ...interface{}) {
	log := plogger.FromContextSafe(ctx).Prefix("PLRWMutex").Tag("mutex")
	s := ""
	if deadlockDetection {
		s = " - using deadlock detection"
		s += fmt.Sprintf(" (%d->%d)", locknum, locknum+1)
	}
	log.Debugf("["+format+"] RLock"+s, args...)
	o.RWMutex.RLock()
	log.Debugf("["+format+"] RLock OK", args...)
}

func (o *PLRWMutex) RUnlock(ctx context.Context, format string, args ...interface{}) {
	log := plogger.FromContextSafe(ctx).Prefix("PLRWMutex").Tag("mutex")
	s := ""
	if deadlockDetection {
		s = " - using deadlock detection"
		s += fmt.Sprintf(" (%d->%d)", locknum, locknum-1)
	}
	log.Debugf("["+format+"] RUnlock"+s, args...)
	o.RWMutex.RUnlock()
	log.Debugf("["+format+"] RUnlock OK", args...)
}

func (o *PLRWMutex) Exec(ctx context.Context, f func(), format string, args ...interface{}) {
	o.Lock(ctx, format, args...)
	f()
	o.Unlock(ctx, format, args...)
}

/*
 * PLMutex is a wrapper around RWMutex
 */
type PLMutex struct {
	Mutex
}

func (o *PLMutex) Lock(ctx context.Context, format string, args ...interface{}) {
	log := plogger.FromContextSafe(ctx).Prefix("PLMutex").Tag("mutex")
	s := ""
	if deadlockDetection {
		s = " - using deadlock detection"
		s += fmt.Sprintf(" (%d->%d)", locknum, locknum+1)
	}

	log.Debugf("["+format+"] Lock"+s, args...)
	o.Mutex.Lock()
	log.Debugf("["+format+"] Lock OK", args...)
}

func (o *PLMutex) Unlock(ctx context.Context, format string, args ...interface{}) {
	log := plogger.FromContextSafe(ctx).Prefix("PLMutex").Tag("mutex")
	s := ""
	if deadlockDetection {
		s = " - using deadlock detection"
		s += fmt.Sprintf(" (%d->%d)", locknum, locknum-1)
	}
	log.Debugf("["+format+"] Unlock"+s, args...)
	o.Mutex.Unlock()
	log.Debugf("["+format+"] Unlock OK", args...)
}

func (o *PLMutex) Exec(ctx context.Context, f func(), format string, args ...interface{}) {
	o.Lock(ctx, format, args...)
	f()
	o.Unlock(ctx, format, args...)
}

type NamedRWMutex struct {
	PLRWMutex
	Name string
}

func (o *NamedRWMutex) Init(format string, args ...interface{}) {
	o.Name = fmt.Sprintf(format, args...)
}

func (o *NamedRWMutex) Lock(ctx context.Context) {
	Assert(func() bool { return o.Name != "" }, "call Init(...)")

	o.PLRWMutex.Lock(ctx, o.Name)
}

func (o *NamedRWMutex) Unlock(ctx context.Context) {
	Assert(func() bool { return o.Name != "" }, "call Init(...)")

	o.PLRWMutex.Unlock(ctx, o.Name)
}

func (o *NamedRWMutex) RLock(ctx context.Context) {
	Assert(func() bool { return o.Name != "" }, "call Init(...)")

	o.PLRWMutex.RLock(ctx, o.Name)
}

func (o *NamedRWMutex) RUnlock(ctx context.Context) {
	Assert(func() bool { return o.Name != "" }, "call Init(...)")

	o.PLRWMutex.RUnlock(ctx, o.Name)
}

type NamedMutex struct {
	PLMutex
	Name string
}

func (o *NamedMutex) Init(name string) {
	o.Name = name
}

func (o *NamedMutex) Lock(ctx context.Context) {
	Assert(func() bool { return o.Name != "" }, "call Init(...)")

	o.PLMutex.Lock(ctx, o.Name)
}

func (o *NamedMutex) Unlock(ctx context.Context) {
	Assert(func() bool { return o.Name != "" }, "call Init(...)")

	o.PLMutex.Unlock(ctx, o.Name)
}
