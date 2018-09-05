package sdp_test

import "fmt"

type testLogger struct{}

func (l *testLogger) Debugf(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}
func (l *testLogger) Infof(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}
func (l *testLogger) Warnf(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}
func (l *testLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}
func (l *testLogger) Fatalf(format string, args ...interface{}) {
	fmt.Printf("[FATAL] "+format+"\n", args...)
}
