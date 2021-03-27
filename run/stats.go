package run

import (
	"time"
)

type Stats struct {
	Address     string
	Item        int
	Bytes       uint64
	ElapsedTime time.Duration
}
