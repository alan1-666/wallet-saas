package logger

import (
	"log"
	"os"
)

func New(prefix string) *log.Logger {
	return log.New(os.Stdout, prefix+" ", log.LstdFlags|log.Lmicroseconds)
}
