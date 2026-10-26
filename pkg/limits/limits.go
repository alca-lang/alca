// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package limits implements the process-wide memory accounting used by the
// Alca runtime.
//
// The guard does not try to catch a Go out-of-memory condition: in Go that is
// a fatal, unrecoverable error that kills the process. The guard therefore
// works ahead of the allocator. It keeps an estimate of the memory an Alca
// program occupies and reports that the limit was exceeded, so the runtime can
// turn it into a regular Alca error that the program itself can catch.
//
// Two numbers are tracked:
//
//   - live is an estimate of the memory that is currently held by Alca data:
//     list elements, map entries, environment bindings, the fixed structures of
//     a virtual machine, and compiled JIT artifacts. It grows when data grows
//     and shrinks again when data is released.
//   - allocated is a cumulative counter of everything that was charged. It
//     also covers transient allocations, such as the intermediate strings of a
//     concatenation loop, that never become reachable and therefore cannot be
//     refunded.
//
// The limits are:
//
//   - the live limit bounds the memory a program may hold at once;
//   - the allocation limit bounds the total amount a program may allocate.
//
// A limit of zero means "no limit". The meter is process-wide, which matches
// the way the command line tool and the standalone runtime execute a program:
// one program per process. Hosts that embed several interpreters in one process
// share the budget and should call Reset between programs.
package limits

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// CheckInterval is the number of ticks between two limit checks. Hot paths
// call Tick on every instruction or node and only read the meter when it
// reports that a check is due.
const CheckInterval = 512

// Costs used when charging the meter. They approximate the Go heap cost of
// the corresponding Alca object, rounded to the size classes the allocator
// actually uses.
const (
	// CostValueSlot is one interface slot inside a container.
	CostValueSlot int64 = 16
	// CostBoxedValue is a boxed primitive stored inside a container.
	CostBoxedValue int64 = 16
	// CostMapEntry is one key/value pair of a map, including the bucket
	// overhead the Go map keeps for it.
	CostMapEntry int64 = 48
	// CostStringByte is one byte of string data.
	CostStringByte int64 = 1
	// CostBinding is one environment binding.
	CostBinding int64 = 32
	// CostClosure is one closure with its captured values.
	CostClosure int64 = 96
)

// Meter tracks the memory an Alca program may use.
type Meter struct {
	mu         sync.Mutex
	liveLimit  int64
	allocLimit int64
	live       int64
	allocated  int64
	peak       int64
	over       bool
	tick       atomic.Int64
}

// Snapshot describes the current state of a meter.
type Snapshot struct {
	LiveLimit  int64
	AllocLimit int64
	Live       int64
	Allocated  int64
	Peak       int64
	Exceeded   bool
}

var defaultMeter = &Meter{}

// Default returns the process-wide meter.
func Default() *Meter { return defaultMeter }

// Tick reports whether the caller should perform a limit check now. It is
// meant for hot paths: the counter is atomic, so it costs one increment, and
// the meter is only read every CheckInterval ticks.
func (m *Meter) Tick() bool {
	return m.tick.Add(1)%CheckInterval == 0
}

// SetLimits installs the live and allocation limits. Zero disables a limit.
func (m *Meter) SetLimits(liveLimit, allocLimit int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.liveLimit = liveLimit
	m.allocLimit = allocLimit
	if m.live > m.liveLimit && m.liveLimit > 0 {
		m.over = true
	}
	if m.allocated > m.allocLimit && m.allocLimit > 0 {
		m.over = true
	}
}

// Limits returns the configured live and allocation limits.
func (m *Meter) Limits() (liveLimit, allocLimit int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.liveLimit, m.allocLimit
}

// Reset clears the counters and the exceeded flag. It does not change the
// configured limits.
func (m *Meter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.live = 0
	m.allocated = 0
	m.peak = 0
	m.over = false
}

// Charge accounts for memory that stays reachable: it updates both the live
// estimate and the cumulative counter.
func (m *Meter) Charge(n int64) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.live += n
	m.allocated += n
	if m.live > m.peak {
		m.peak = m.live
	}
	m.checkLocked()
}

// ChargeTransient accounts for memory that is allocated and released again,
// such as the intermediate result of a string concatenation. It only updates
// the cumulative counter.
func (m *Meter) ChargeTransient(n int64) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allocated += n
	m.checkLocked()
}

// Refund returns memory that was released, for example a popped list element
// or a deleted map entry. The cumulative counter is not decreased: the memory
// was still allocated at some point.
func (m *Meter) Refund(n int64) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.live -= n
	if m.live < 0 {
		m.live = 0
	}
}

// checkLocked updates the exceeded flag. The caller holds the lock.
func (m *Meter) checkLocked() {
	if m.liveLimit > 0 && m.live > m.liveLimit {
		m.over = true
	}
	if m.allocLimit > 0 && m.allocated > m.allocLimit {
		m.over = true
	}
}

// Exceeded reports whether a limit was reached. The flag stays set until the
// meter is reset, so a program that caught the error cannot silently continue
// to grow past the limit.
func (m *Meter) Exceeded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.over
}

// Live returns the current estimate of reachable Alca memory.
func (m *Meter) Live() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.live
}

// Allocated returns the cumulative number of charged bytes.
func (m *Meter) Allocated() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.allocated
}

// Peak returns the highest live estimate reached so far.
func (m *Meter) Peak() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.peak
}

// State returns a copy of the current counters.
func (m *Meter) State() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Snapshot{
		LiveLimit:  m.liveLimit,
		AllocLimit: m.allocLimit,
		Live:       m.live,
		Allocated:  m.allocated,
		Peak:       m.peak,
		Exceeded:   m.over,
	}
}

// Message describes the exceeded limit for an error message.
func (m *Meter) Message() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case m.liveLimit > 0 && m.live > m.liveLimit:
		return fmt.Sprintf("out of memory (the program holds more than the %d byte limit)", m.liveLimit)
	case m.allocLimit > 0 && m.allocated > m.allocLimit:
		return fmt.Sprintf("out of memory (the program allocated more than the %d byte limit)", m.allocLimit)
	default:
		return "out of memory"
	}
}

// SetLimits installs the limits on the process-wide meter.
func SetLimits(liveLimit, allocLimit int64) { defaultMeter.SetLimits(liveLimit, allocLimit) }

// Limits returns the limits of the process-wide meter.
func Limits() (liveLimit, allocLimit int64) { return defaultMeter.Limits() }

// Reset clears the counters of the process-wide meter.
func Reset() { defaultMeter.Reset() }

// Charge accounts reachable memory on the process-wide meter.
func Charge(n int64) { defaultMeter.Charge(n) }

// ChargeTransient accounts transient memory on the process-wide meter.
func ChargeTransient(n int64) { defaultMeter.ChargeTransient(n) }

// Refund returns released memory on the process-wide meter.
func Refund(n int64) { defaultMeter.Refund(n) }

// Exceeded reports whether a limit was reached on the process-wide meter.
func Exceeded() bool { return defaultMeter.Exceeded() }

// Tick reports whether the caller should perform a limit check now.
func Tick() bool { return defaultMeter.Tick() }

// Message describes the exceeded limit of the process-wide meter.
func Message() string { return defaultMeter.Message() }

// State returns the counters of the process-wide meter.
func State() Snapshot { return defaultMeter.State() }

// ParseSize converts a size such as "512", "64kb", "256mb" or "2gb" into a
// number of bytes. It understands the same suffixes as the @oom_guard
// decorator. A size of zero means "no limit".
func ParseSize(raw string) (int64, error) {
	raw = strings.TrimSpace(strings.ToUpper(raw))
	if raw == "" {
		return 0, fmt.Errorf("empty size")
	}
	multiplier := int64(1)
	switch {
	case strings.HasSuffix(raw, "GB"):
		multiplier = 1024 * 1024 * 1024
		raw = strings.TrimSuffix(raw, "GB")
	case strings.HasSuffix(raw, "MB"):
		multiplier = 1024 * 1024
		raw = strings.TrimSuffix(raw, "MB")
	case strings.HasSuffix(raw, "KB"):
		multiplier = 1024
		raw = strings.TrimSuffix(raw, "KB")
	case strings.HasSuffix(raw, "B"):
		raw = strings.TrimSuffix(raw, "B")
	}
	num, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", raw)
	}
	if num < 0 {
		return 0, fmt.Errorf("invalid size %q", raw)
	}
	return num * multiplier, nil
}
