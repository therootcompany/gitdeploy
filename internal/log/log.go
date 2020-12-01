package log

import (
	"log"
)

// Printf wraps log.Printf
func Printf(msg string, els ...interface{}) {
	log.Printf(msg, els...)
}
