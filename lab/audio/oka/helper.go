package main

import (
	"log"
	"runtime"
)

func fatal(args ...any) {
	if len(args) == 0 {
		return
	}
	for _, arg := range args {
		if err, ok := arg.(error); ok && err != nil {
			_, file, line, _ := runtime.Caller(1)
			log.Fatalf("[%s:%d] %v", file, line, err)
		}
	}
}
