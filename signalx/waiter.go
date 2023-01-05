package signalx

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/go-leo/syncx"
	"golang.org/x/sync/errgroup"
)

type SignalHook = map[os.Signal]func()

type SignalWaiter struct {
	signals        []os.Signal
	signalC        chan os.Signal
	incomingSignal os.Signal
	hooks          []func(os.Signal)
	waitTimeout    time.Duration
}

func NewSignalWaiter(signals []os.Signal, waitTimeout time.Duration, hooks ...func(os.Signal)) *SignalWaiter {
	w := &SignalWaiter{
		signals:        signals,
		signalC:        make(chan os.Signal),
		incomingSignal: nil,
		hooks:          hooks,
		waitTimeout:    waitTimeout,
	}
	signal.Notify(w.signalC, w.signals...)
	return w
}

func (w *SignalWaiter) Wait() *SignalWaiter {
	// 监听一个系统信号
	w.incomingSignal = <-w.signalC

	// 并发调用所有hook
	errGroup, ctx := errgroup.WithContext(context.Background())
	for _, hook := range w.hooks {
		errGroup.Go(func() error {
			syncx.BraveDo(func() { hook(w.Signal()) }, func(p any) {})
			return nil
		})
	}
	go func() { _ = errGroup.Wait() }()

	// 等待退出
	select {
	case w.incomingSignal = <-w.signalC:
		// 如果再接收到一个信息就直接退出。
		return w
	case <-ctx.Done():
		// 等待hook执行完毕后退出
		return w
	case <-time.After(w.waitTimeout):
		// 等待超时退出
		return w
	}
}

func (w *SignalWaiter) Signal() os.Signal {
	return w.incomingSignal
}

func (w *SignalWaiter) Err() error {
	if w.Signal() == nil {
		return nil
	}
	return &SignalError{Signal: w.Signal()}
}
