package coordinator

import "time"

const (
	shutdownTimeout  = 5 * time.Second
	defaultMaxMisses = 1
	scanInterval     = 10 * time.Second
)
