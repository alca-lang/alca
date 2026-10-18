package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Now().UnixNano()
	var i int64 = 0
	var total int64 = 0
	for i < 1000000 {
		total += (i % 7) * 3
		i++
	}
	elapsed := float64(time.Now().UnixNano()-start) / 1e6
	fmt.Printf("Alca Native AOT (1M loop): total=%d, time=%.2f ms\n", total, elapsed)
}
