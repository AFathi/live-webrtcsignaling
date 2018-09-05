package srtp

import (
	"fmt"
	"log"
)

func logString(format string, v ...interface{}) {
	format = format + "\n"
	fmt.Printf(format, v...)
}

func logOnError(err error, format string, v ...interface{}) (status bool) {
	status = false
	format = format + ": %s"
	if err != nil {
		v = append(v, err)
		log.Printf(format, v...)
		status = true
	}

	return
}
