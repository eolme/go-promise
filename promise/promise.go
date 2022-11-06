package promise

import (
	"errors"
	"fmt"
	"sync/atomic"
)

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type internalStatus = uint32

const (
	internalPending   internalStatus = 0
	internalFulfilled internalStatus = 1
	internalRejected  internalStatus = 2
)

type Promise struct {
	noCopy    noCopy
	wait      chan uint32
	status    internalStatus
	rejected  error
	fulfilled any
}

type PromiseAggregateError struct {
	error
	Errors []error
}

type (
	PromiseResolve func(result any)
	PromiseReject  func(reason error)
	PromiseThen    func(result any) (any, error)
	PromiseCatch   func(reason error) (any, error)
	PromiseFinally func() error
	PromiseStatus  string
	PromiseSettled struct {
		Status PromiseStatus
		Reason error
		Value  any
	}
)

const (
	PromiseStatusFulfilled PromiseStatus = "fulfilled"
	PromiseStatusRejected  PromiseStatus = "rejected"
)

func New(fn func(resolve PromiseResolve, reject PromiseReject)) (promise *Promise) {
	promise = createPromise()

	fn(func(result any) {
		assignPromise(promise, result)
	}, func(reason error) {
		rejectPromise(promise, reason)
	})

	return promise
}

func Resolve(result any) (promise *Promise) {
	promise = createPromise()

	go assignPromise(promise, result)

	return promise
}

func Reject(reason error) (promise *Promise) {
	promise = createPromise()

	go rejectPromise(promise, reason)

	return promise
}

func All(arr []any) (promise *Promise) {
	promise = createPromise()

	go func() {
		count := uint32(0)
		length := uint32(len(arr))

		rejected := uint32(0)
		var err error = nil

		all := make([]any, length)

		for index := range arr {
			go func(index int) {
				unpacked, reason := unpackPromise(arr[index])
				atomic.AddUint32(&count, 1)

				if reason != nil {
					if atomic.CompareAndSwapUint32(&rejected, 0, 1) {
						err = reason
					}
				} else {
					all[index] = unpacked
				}

				if atomic.LoadUint32(&count) == length {
					if atomic.LoadUint32(&rejected) == 1 {
						rejectPromise(promise, err)
					} else {
						fulfillPromise(promise, all)
					}
				}
			}(index)
		}
	}()

	return promise
}

func Race(arr []any) (promise *Promise) {
	promise = createPromise()

	go func() {
		count := uint32(0)
		length := uint32(len(arr))

		rejected := uint32(0)
		var err error = nil

		resolved := uint32(0)
		var race any = nil

		for index := range arr {
			go func(index int) {
				unpacked, reason := unpackPromise(arr[index])
				atomic.AddUint32(&count, 1)

				if reason != nil {
					if atomic.LoadUint32(&rejected) == 0 {
						atomic.StoreUint32(&rejected, 1)
						err = reason
					}
				} else {
					if atomic.LoadUint32(&resolved) == 0 {
						atomic.StoreUint32(&resolved, 1)
						race = unpacked
					}
				}

				if atomic.LoadUint32(&count) == length {
					if atomic.LoadUint32(&rejected) == 1 {
						rejectPromise(promise, err)
					} else {
						fulfillPromise(promise, race)
					}
				}
			}(index)
		}
	}()

	return promise
}

func Any(arr []any) (promise *Promise) {
	promise = createPromise()

	go func() {
		count := uint32(0)
		length := uint32(len(arr))

		errors := make([]error, length)

		for index := range arr {
			go func(index int) {
				unpacked, reason := unpackPromise(arr[index])
				atomic.AddUint32(&count, 1)

				errors[index] = reason

				if reason == nil {
					fulfillPromise(promise, unpacked)
				}

				if atomic.LoadUint32(&count) == length {
					if atomic.LoadUint32(&promise.status) == internalPending {
						aggregated := []error{}

						for _, err := range errors {
							if err != nil {
								aggregated = append(aggregated, err)
							}
						}

						rejectPromise(promise, PromiseAggregateError{
							Errors: aggregated,
						})
					}
				}
			}(index)
		}
	}()

	return promise
}

func AllSettled(arr []any) (promise *Promise) {
	promise = createPromise()

	go func() {
		count := uint32(0)
		length := uint32(len(arr))

		settled := make([]PromiseSettled, length)

		for index := range arr {
			go func(index int) {
				unpacked, reason := unpackPromise(arr[index])
				atomic.AddUint32(&count, 1)

				var status PromiseStatus
				if reason == nil {
					status = PromiseStatusFulfilled
				} else {
					status = PromiseStatusRejected
				}

				settled[index] = PromiseSettled{
					Status: status,
					Reason: reason,
					Value:  unpacked,
				}

				if atomic.LoadUint32(&count) == length {
					fulfillPromise(promise, settled)
				}
			}(index)
		}
	}()

	return promise
}

func (self *Promise) Then(then PromiseThen) (promise *Promise) {
	promise = createPromise()

	go func() {
		<-self.wait

		switch atomic.LoadUint32(&self.status) {
		case internalFulfilled:
			packed, reason := then(self.fulfilled)
			resolvePromise(promise, packed, reason)
		case internalRejected:
			rejectPromise(promise, self.rejected)
		}
	}()

	return promise
}

func (self *Promise) Catch(catch PromiseCatch) (promise *Promise) {
	promise = createPromise()

	go func() {
		<-self.wait

		switch atomic.LoadUint32(&self.status) {
		case internalFulfilled:
			fulfillPromise(promise, self.fulfilled)
		case internalRejected:
			packed, reason := catch(self.rejected)
			resolvePromise(promise, packed, reason)
		}
	}()

	return promise
}

func (self *Promise) ThenCatch(then PromiseThen, catch PromiseCatch) (promise *Promise) {
	promise = createPromise()

	go func() {
		<-self.wait

		switch atomic.LoadUint32(&self.status) {
		case internalFulfilled:
			packed, reason := then(self.fulfilled)
			resolvePromise(promise, packed, reason)
		case internalRejected:
			packed, reason := catch(self.rejected)
			resolvePromise(promise, packed, reason)
		}
	}()

	return promise
}

func (self *Promise) Finally(finally PromiseFinally) (promise *Promise) {
	promise = createPromise()

	go func() {
		<-self.wait

		err := finally()

		if err != nil {
			rejectPromise(promise, err)
		} else {
			switch atomic.LoadUint32(&self.status) {
			case internalFulfilled:
				fulfillPromise(promise, self.fulfilled)
			case internalRejected:
				rejectPromise(promise, self.rejected)
			}
		}
	}()

	return promise
}

func (self *Promise) Wait() (unpacked any, reason error) {
	<-self.wait
	return self.fulfilled, self.rejected
}

func createPromise() (promise *Promise) {
	promise = &Promise{
		wait:      make(chan uint32, 0),
		status:    internalPending,
		fulfilled: nil,
		rejected:  nil,
	}

	atomic.StoreUint32(&promise.status, internalPending)

	return promise
}

func unpackPromise(value any) (result any, reason error) {
	if promise, ok := value.(*Promise); ok {
		<-promise.wait

		switch atomic.LoadUint32(&promise.status) {
		case internalFulfilled:
			return unpackPromise(promise.fulfilled)
		case internalRejected:
			return nil, promise.rejected
		}
	}

	return value, nil
}

func fulfillPromise(promise *Promise, unpacked any) {
	if atomic.CompareAndSwapUint32(&promise.status, internalPending, internalFulfilled) {
		promise.fulfilled = unpacked
		close(promise.wait)
	}
}

func rejectPromise(promise *Promise, reason error) {
	if atomic.CompareAndSwapUint32(&promise.status, internalPending, internalRejected) {
		promise.rejected = reason
		close(promise.wait)
	}
}

func assignPromise(promise *Promise, packed any) {
	unpacked, reason := unpackPromise(packed)
	if reason != nil {
		rejectPromise(promise, reason)
	} else {
		fulfillPromise(promise, unpacked)
	}
}

func resolvePromise(promise *Promise, packed any, reason error) {
	if packed == promise {
		rejectPromise(promise, errors.New(fmt.Sprintf("Chaining cycle detected for promise %p", promise)))
	} else {
		if reason != nil {
			rejectPromise(promise, reason)
		} else {
			assignPromise(promise, packed)
		}
	}
}
