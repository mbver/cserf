package serf

import "sync/atomic"

type LamportTime uint64

type LamportClock struct {
	time uint64
}

func (l *LamportClock) Time() LamportTime {
	return LamportTime(atomic.LoadUint64(&l.time))
}

func (l *LamportClock) Next() LamportTime {
	return LamportTime(atomic.AddUint64(&l.time, 1))
}

func (l *LamportClock) Witness(t LamportTime) {
	for {
		t0 := uint64(l.Time())
		t1 := uint64(t)
		if t0 > t1 {
			break
		}
		if atomic.CompareAndSwapUint64(&l.time, t0, t1+1) {
			break
		}
	}
}
