package my

// shouldn't race (init only func)
var enabled = false

func EnableAssert() {
	enabled = true
}

// offensive programming
func Assert(f func() bool, s string) {
	if !enabled {
		return
	}
	if f() != true {
		// FIXME: stack trace.
		panic("Assert failed: " + s)
	}
}
