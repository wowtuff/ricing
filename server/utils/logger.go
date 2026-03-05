package utils

import (
	"fmt"
	"os"
)

var f, _ = os.OpenFile("../app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)

func LogError(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	f.WriteString(msg + "\n")
	return fmt.Errorf("%s", msg)
}
