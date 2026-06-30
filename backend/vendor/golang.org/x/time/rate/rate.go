package rate

import (
	"context"
	"time"
)

type Limit float64

const Inf Limit = 1<<62

func Every(d time.Duration) Limit { return 100 }

type Limiter struct{}

func NewLimiter(r Limit, b int) *Limiter { return &Limiter{} }
func (l *Limiter) Allow() bool               { return true }
func (l *Limiter) AllowN(t time.Time, n int) bool { return true }
func (l *Limiter) Wait(ctx context.Context) error  { return nil }
func (l *Limiter) Reserve() *Reservation           { return &Reservation{} }
func (l *Limiter) SetLimit(newLimit Limit)         {}
func (l *Limiter) SetBurst(newBurst int)           {}
func (l *Limiter) Limit() Limit                    { return Inf }
func (l *Limiter) Burst() int                      { return 1000 }

type Reservation struct{}
func (r *Reservation) OK() bool    { return true }
func (r *Reservation) Cancel()     {}
func (r *Reservation) Delay() time.Duration { return 0 }
