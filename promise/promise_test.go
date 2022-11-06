package promise_test

import (
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	deferred "github.com/eolme/go-promise/deferred"
	promise "github.com/eolme/go-promise/promise"
)

const SYNC = false

func createDummyValue() any {
	return struct {
		Value int
	}{
		Value: rand.Int(),
	}
}

func createDummyReason() error {
	return errors.New(strconv.Itoa(rand.Int()))
}

func assertEqual(t *testing.T, actual any, expected any) {
	if actual == expected {
		t.Logf("Assertion success: expected (%T)%v, received (%T)%v", expected, expected, actual, actual)
	} else {
		t.Errorf("Assertion fail: expected (%T)%v, received (%T)%v", expected, expected, actual, actual)
	}
}

func assertError(t *testing.T, actual error, expected string) {
	reason := actual.Error()
	if strings.Contains(reason, expected) {
		t.Logf("Assertion success: expected error `%s`, received `%s`", expected, reason)
	} else {
		t.Errorf("Assertion fail: expected error `%s`, received `%s`", expected, reason)
	}
}

func timeout(ms time.Duration, f func()) {
	time.AfterFunc(time.Duration(time.Millisecond*ms), f)
}

func callbackAggregator(times uint32, callback func()) func() {
	current := uint32(0)

	return func() {
		if atomic.AddUint32(&current, 1) == times {
			callback()
		}
	}
}

func testPrepare(t *testing.T) {
	if !SYNC {
		t.Parallel()
	}
}

func testSync(t *testing.T, name string, sync func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		testPrepare(t)
		sync(t)
	})
}

func testAsync(t *testing.T, name string, async func(t *testing.T, done func())) {
	testSync(t, name, func(t *testing.T) {
		wait := sync.WaitGroup{}
		wait.Add(1)

		called := uint32(0)

		go async(t, func() {
			if atomic.LoadUint32(&called) == 0 {
				atomic.StoreUint32(&called, 1)
				wait.Done()
			} else {
				t.Logf("Done called multiple times in `%s`", t.Name())
			}
		})

		wait.Wait()
	})
}

func testFulfilled(t *testing.T, value any, test func(t *testing.T, instance *promise.Promise, done func())) {
	testAsync(t, "already-fulfilled", func(t *testing.T, done func()) {
		test(t, promise.Resolve(value), done)
	})

	testAsync(t, "immediately-fulfilled", func(t *testing.T, done func()) {
		deferred := deferred.New()
		test(t, deferred.Promise, done)
		deferred.Resolve(value)
	})

	testAsync(t, "eventually-fulfilled", func(t *testing.T, done func()) {
		deferred := deferred.New()
		test(t, deferred.Promise, done)
		timeout(50, func() {
			deferred.Resolve(value)
		})
	})
}

func testRejected(t *testing.T, reason error, test func(t *testing.T, instance *promise.Promise, done func())) {
	testAsync(t, "already-rejected", func(t *testing.T, done func()) {
		test(t, promise.Reject(reason), done)
	})

	testAsync(t, "immediately-rejected", func(t *testing.T, done func()) {
		deferred := deferred.New()
		test(t, deferred.Promise, done)
		deferred.Reject(reason)
	})

	testAsync(t, "eventually-rejected", func(t *testing.T, done func()) {
		deferred := deferred.New()
		test(t, deferred.Promise, done)
		timeout(50, func() {
			deferred.Reject(reason)
		})
	})
}

func Test2121(t *testing.T) {
	testPrepare(t)

	testFulfilledValue := createDummyValue()
	testFulfilled(t, testFulfilledValue, func(t *testing.T, instance *promise.Promise, done func()) {
		onFulfilledCalled := false

		instance.ThenCatch(func(result any) (any, error) {
			onFulfilledCalled = true

			return nil, nil
		}, func(reason error) (any, error) {
			assertEqual(t, onFulfilledCalled, false)
			done()

			return nil, nil
		})

		timeout(100, done)
	})

	testAsync(t, "trying to fulfill then immediately reject", func(t *testing.T, done func()) {
		dummyValue := createDummyValue()
		dummyReason := createDummyReason()
		deferred := deferred.New()
		onFulfilledCalled := false

		deferred.Promise.ThenCatch(func(result any) (any, error) {
			onFulfilledCalled = true

			return nil, nil
		}, func(reason error) (any, error) {
			assertEqual(t, onFulfilledCalled, false)
			done()

			return nil, nil
		})

		deferred.Resolve(dummyValue)
		deferred.Reject(dummyReason)

		timeout(100, done)
	})

	testAsync(t, "trying to fulfill then reject, delayed", func(t *testing.T, done func()) {
		dummyValue := createDummyValue()
		dummyReason := createDummyReason()
		deferred := deferred.New()
		onFulfilledCalled := false

		deferred.Promise.ThenCatch(func(result any) (any, error) {
			onFulfilledCalled = true

			return nil, nil
		}, func(reason error) (any, error) {
			assertEqual(t, onFulfilledCalled, false)

			return nil, nil
		})

		timeout(50, func() {
			deferred.Resolve(dummyValue)
			deferred.Reject(dummyReason)
		})
		timeout(100, done)
	})

	testAsync(t, "trying to fulfill immediately then reject delayed", func(t *testing.T, done func()) {
		dummyValue := createDummyValue()
		dummyReason := createDummyReason()
		deferred := deferred.New()
		onFulfilledCalled := false

		deferred.Promise.ThenCatch(func(result any) (any, error) {
			onFulfilledCalled = true

			return nil, nil
		}, func(reason error) (any, error) {
			assertEqual(t, onFulfilledCalled, false)
			done()

			return nil, nil
		})

		deferred.Resolve(dummyValue)
		timeout(50, func() {
			deferred.Reject(dummyReason)
		})
		timeout(100, done)
	})
}

func Test2131(t *testing.T) {
	testPrepare(t)

	testRejectedReason := createDummyReason()
	testRejected(t, testRejectedReason, func(t *testing.T, instance *promise.Promise, done func()) {
		onRejectedCalled := false

		instance.ThenCatch(func(result any) (any, error) {
			assertEqual(t, onRejectedCalled, false)
			done()

			return nil, nil
		}, func(reason error) (any, error) {
			onRejectedCalled = true

			return nil, nil
		})

		timeout(100, done)
	})

	testAsync(t, "trying to reject then immediately fulfill", func(t *testing.T, done func()) {
		dummyValue := createDummyValue()
		dummyReason := createDummyReason()
		deferred := deferred.New()
		onRejectedCalled := false

		deferred.Promise.ThenCatch(func(result any) (any, error) {
			assertEqual(t, onRejectedCalled, false)
			done()

			return nil, nil
		}, func(reason error) (any, error) {
			onRejectedCalled = true

			return nil, nil
		})

		deferred.Reject(dummyReason)
		deferred.Resolve(dummyValue)

		timeout(100, done)
	})

	testAsync(t, "trying to reject then fulfill, delayed", func(t *testing.T, done func()) {
		dummyValue := createDummyValue()
		dummyReason := createDummyReason()
		deferred := deferred.New()
		onRejectedCalled := false

		deferred.Promise.ThenCatch(func(result any) (any, error) {
			assertEqual(t, onRejectedCalled, false)
			done()

			return nil, nil
		}, func(reason error) (any, error) {
			onRejectedCalled = true

			return nil, nil
		})

		timeout(50, func() {
			deferred.Reject(dummyReason)
			deferred.Resolve(dummyValue)
		})
		timeout(100, done)
	})

	testAsync(t, "trying to fulfill immediately then reject delayed", func(t *testing.T, done func()) {
		dummyValue := createDummyValue()
		dummyReason := createDummyReason()
		deferred := deferred.New()
		onRejectedCalled := false

		deferred.Promise.ThenCatch(func(result any) (any, error) {
			assertEqual(t, onRejectedCalled, false)
			done()

			return nil, nil
		}, func(reason error) (any, error) {
			onRejectedCalled = true

			return nil, nil
		})

		deferred.Reject(dummyReason)
		timeout(50, func() {
			deferred.Resolve(dummyValue)
		})
		timeout(100, done)
	})
}

func Test221(t *testing.T) {
	testPrepare(t)

	t.Skip("impossible because implementation is strictly typified")
}

func Test2221(t *testing.T) {
	testPrepare(t)

	testFulfilledValue := createDummyValue()
	testFulfilled(t, testFulfilledValue, func(t *testing.T, instance *promise.Promise, done func()) {
		instance.Then(func(result any) (any, error) {
			assertEqual(t, result, testFulfilledValue)
			done()

			return nil, nil
		})
	})
}

func Test2222(t *testing.T) {
	testPrepare(t)

	testAsync(t, "fulfilled after a delay", func(t *testing.T, done func()) {
		dummyValue := createDummyValue()
		deferred := deferred.New()
		isFulfilled := false

		deferred.Promise.Then(func(_ any) (any, error) {
			assertEqual(t, isFulfilled, true)
			done()

			return nil, nil
		})

		timeout(50, func() {
			deferred.Resolve(dummyValue)
			isFulfilled = true
		})
	})

	testAsync(t, "never fulfilled", func(t *testing.T, done func()) {
		deferred := deferred.New()
		onFulfilledCalled := false

		deferred.Promise.Then(func(_ any) (any, error) {
			onFulfilledCalled = true
			done()

			return nil, nil
		})

		timeout(150, func() {
			assertEqual(t, onFulfilledCalled, false)
			done()
		})
	})
}

func Test2223(t *testing.T) {
	testPrepare(t)

	testAsync(t, "already-fulfilled", func(t *testing.T, done func()) {
		dummyValue := createDummyValue()
		timesCalled := uint32(0)

		promise.Resolve(dummyValue).Then(func(_ any) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled, 1), uint32(1))
			done()

			return nil, nil
		})
	})

	testAsync(t, "trying to fulfill a pending promise more than once, immediately", func(t *testing.T, done func()) {
		deferred := deferred.New()
		dummyValue := createDummyValue()
		timesCalled := uint32(0)

		deferred.Promise.Then(func(_ any) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled, 1), uint32(1))
			done()

			return nil, nil
		})

		deferred.Resolve(dummyValue)
		deferred.Resolve(dummyValue)
	})

	testAsync(t, "trying to fulfill a pending promise more than once, delayed", func(t *testing.T, done func()) {
		deferred := deferred.New()
		dummyValue := createDummyValue()
		timesCalled := uint32(0)

		deferred.Promise.Then(func(_ any) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled, 1), uint32(1))
			done()

			return nil, nil
		})

		timeout(50, func() {
			deferred.Resolve(dummyValue)
			deferred.Resolve(dummyValue)
		})
	})

	testAsync(t, "trying to fulfill a pending promise more than once, immediately then delayed", func(t *testing.T, done func()) {
		deferred := deferred.New()
		dummyValue := createDummyValue()
		timesCalled := uint32(0)

		deferred.Promise.Then(func(_ any) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled, 1), uint32(1))
			done()

			return nil, nil
		})

		deferred.Resolve(dummyValue)
		timeout(50, func() {
			deferred.Resolve(dummyValue)
		})
	})

	testAsync(t, "when multiple `then` calls are made, spaced apart in time", func(t *testing.T, done func()) {
		deferred := deferred.New()
		dummyValue := createDummyValue()
		timesCalled1 := uint32(0)
		timesCalled2 := uint32(0)
		timesCalled3 := uint32(0)

		deferred.Promise.Then(func(_ any) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled1, 1), uint32(1))

			return nil, nil
		})

		timeout(50, func() {
			deferred.Promise.Then(func(_ any) (any, error) {
				assertEqual(t, atomic.AddUint32(&timesCalled2, 1), uint32(1))

				return nil, nil
			})
		})

		timeout(100, func() {
			deferred.Promise.Then(func(_ any) (any, error) {
				assertEqual(t, atomic.AddUint32(&timesCalled3, 1), uint32(1))
				done()

				return nil, nil
			})
		})

		timeout(150, func() {
			deferred.Resolve(dummyValue)
		})
	})

	testAsync(t, "when `then` is interleaved with fulfillment", func(t *testing.T, done func()) {
		deferred := deferred.New()
		dummyValue := createDummyValue()
		timesCalled1 := uint32(0)
		timesCalled2 := uint32(0)

		deferred.Promise.Then(func(_ any) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled1, 1), uint32(1))

			return nil, nil
		})

		deferred.Resolve(dummyValue)

		deferred.Promise.Then(func(_ any) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled2, 1), uint32(1))
			done()

			return nil, nil
		})
	})
}

func Test2231(t *testing.T) {
	testPrepare(t)

	testRejectedValue := createDummyReason()
	testRejected(t, testRejectedValue, func(t *testing.T, instance *promise.Promise, done func()) {
		instance.Catch(func(reason error) (any, error) {
			assertEqual(t, reason, testRejectedValue)
			done()

			return nil, nil
		})
	})
}

func Test2232(t *testing.T) {
	testPrepare(t)

	testAsync(t, "rejected after a delay", func(t *testing.T, done func()) {
		dummyReason := createDummyReason()
		deferred := deferred.New()
		isRejected := false

		deferred.Promise.Catch(func(_ error) (any, error) {
			assertEqual(t, isRejected, true)
			done()

			return nil, nil
		})

		timeout(50, func() {
			deferred.Reject(dummyReason)
			isRejected = true
		})
	})

	testAsync(t, "never rejected", func(t *testing.T, done func()) {
		deferred := deferred.New()
		onRejectedCalled := false

		deferred.Promise.Catch(func(_ error) (any, error) {
			onRejectedCalled = true
			done()

			return nil, nil
		})

		timeout(150, func() {
			assertEqual(t, onRejectedCalled, false)
			done()
		})
	})
}

func Test2233(t *testing.T) {
	testPrepare(t)

	testAsync(t, "already-rejected", func(t *testing.T, done func()) {
		dummyReason := createDummyReason()
		timesCalled := uint32(0)

		promise.Reject(dummyReason).Catch(func(_ error) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled, 1), uint32(1))
			done()

			return nil, nil
		})
	})

	testAsync(t, "trying to reject a pending promise more than once, immediately", func(t *testing.T, done func()) {
		deferred := deferred.New()
		dummyReason := createDummyReason()
		timesCalled := uint32(0)

		deferred.Promise.Catch(func(_ error) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled, 1), uint32(1))
			done()

			return nil, nil
		})

		deferred.Reject(dummyReason)
		deferred.Reject(dummyReason)
	})

	testAsync(t, "trying to reject a pending promise more than once, delayed", func(t *testing.T, done func()) {
		deferred := deferred.New()
		dummyReason := createDummyReason()
		timesCalled := uint32(0)

		deferred.Promise.Catch(func(_ error) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled, 1), uint32(1))
			done()

			return nil, nil
		})

		timeout(50, func() {
			deferred.Reject(dummyReason)
			deferred.Reject(dummyReason)
		})
	})

	testAsync(t, "trying to fulfill a pending promise more than once, immediately then delayed", func(t *testing.T, done func()) {
		deferred := deferred.New()
		dummyReason := createDummyReason()
		timesCalled := uint32(0)

		deferred.Promise.Catch(func(_ error) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled, 1), uint32(1))
			done()

			return nil, nil
		})

		deferred.Reject(dummyReason)
		timeout(50, func() {
			deferred.Reject(dummyReason)
		})
	})

	testAsync(t, "when multiple `catch` calls are made, spaced apart in time", func(t *testing.T, done func()) {
		deferred := deferred.New()
		dummyReason := createDummyReason()
		timesCalled1 := uint32(0)
		timesCalled2 := uint32(0)
		timesCalled3 := uint32(0)

		deferred.Promise.Catch(func(_ error) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled1, 1), uint32(1))

			return nil, nil
		})

		timeout(50, func() {
			deferred.Promise.Catch(func(_ error) (any, error) {
				assertEqual(t, atomic.AddUint32(&timesCalled2, 1), uint32(1))

				return nil, nil
			})
		})

		timeout(100, func() {
			deferred.Promise.Catch(func(_ error) (any, error) {
				assertEqual(t, atomic.AddUint32(&timesCalled3, 1), uint32(1))
				done()

				return nil, nil
			})
		})

		timeout(150, func() {
			deferred.Reject(dummyReason)
		})
	})

	testAsync(t, "when `catch` is interleaved with rejection", func(t *testing.T, done func()) {
		deferred := deferred.New()
		dummyReason := createDummyReason()
		timesCalled1 := uint32(0)
		timesCalled2 := uint32(0)

		deferred.Promise.Catch(func(_ error) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled1, 1), uint32(1))

			return nil, nil
		})

		deferred.Reject(dummyReason)

		deferred.Promise.Catch(func(_ error) (any, error) {
			assertEqual(t, atomic.AddUint32(&timesCalled2, 1), uint32(1))
			done()

			return nil, nil
		})
	})
}

func Test224(t *testing.T) {
	testPrepare(t)

	testSync(t, "`then` returns before the promise becomes fulfilled or rejected", func(t *testing.T) {
		testFulfilledValue := createDummyValue()
		testFulfilled(t, testFulfilledValue, func(t *testing.T, instance *promise.Promise, done func()) {
			thenHasReturned := false

			instance.Then(func(_ any) (any, error) {
				assertEqual(t, thenHasReturned, true)
				done()

				return nil, nil
			})

			thenHasReturned = true
		})

		testRejectedValue := createDummyReason()
		testRejected(t, testRejectedValue, func(t *testing.T, instance *promise.Promise, done func()) {
			thenHasReturned := false

			instance.Catch(func(_ error) (any, error) {
				assertEqual(t, thenHasReturned, true)
				done()

				return nil, nil
			})

			thenHasReturned = true
		})
	})

	testSync(t, "Clean-stack execution ordering tests (fulfillment case)", func(t *testing.T) {
		testSync(t, "when `onFulfilled` is added immediately before the promise is fulfilled", func(t *testing.T) {
			deferred := deferred.New()
			dummyValue := createDummyValue()
			onFulfilledCalled := false

			deferred.Promise.Then(func(_ any) (any, error) {
				onFulfilledCalled = true

				return nil, nil
			})

			deferred.Resolve(dummyValue)

			assertEqual(t, onFulfilledCalled, false)
		})

		testSync(t, "when `onFulfilled` is added immediately after the promise is fulfilled", func(t *testing.T) {
			deferred := deferred.New()
			dummyValue := createDummyValue()
			onFulfilledCalled := false

			deferred.Resolve(dummyValue)

			deferred.Promise.Then(func(_ any) (any, error) {
				onFulfilledCalled = true

				return nil, nil
			})

			assertEqual(t, onFulfilledCalled, false)
		})

		testAsync(t, "when one `onFulfilled` is added inside another `onFulfilled`", func(t *testing.T, done func()) {
			instance := promise.Resolve(nil)
			firstOnFulfilledFinished := false

			instance.Then(func(result any) (any, error) {
				instance.Then(func(result any) (any, error) {
					assertEqual(t, firstOnFulfilledFinished, true)
					done()

					return nil, nil
				})

				firstOnFulfilledFinished = true

				return nil, nil
			})
		})

		testAsync(t, "when `onFulfilled` is added inside an `onRejected`", func(t *testing.T, done func()) {
			instance1 := promise.Reject(nil)
			instance2 := promise.Resolve(nil)
			firstOnRejectedFinished := false

			instance1.Catch(func(_ error) (any, error) {
				instance2.Then(func(_ any) (any, error) {
					assertEqual(t, firstOnRejectedFinished, true)
					done()

					return nil, nil
				})

				firstOnRejectedFinished = true

				return nil, nil
			})
		})

		testAsync(t, "when the promise is fulfilled asynchronously", func(t *testing.T, done func()) {
			deferred := deferred.New()
			dummyValue := createDummyValue()
			firstStackFinished := false

			timeout(0, func() {
				deferred.Resolve(dummyValue)
				firstStackFinished = true
			})

			deferred.Promise.Then(func(_ any) (any, error) {
				assertEqual(t, firstStackFinished, true)
				done()

				return nil, nil
			})
		})
	})

	testSync(t, "Clean-stack execution ordering tests (rejection case)", func(t *testing.T) {
		testSync(t, "when `onRejected` is added immediately before the promise is rejected", func(t *testing.T) {
			deferred := deferred.New()
			dummyReason := createDummyReason()
			onRejectedCalled := false

			deferred.Promise.Catch(func(_ error) (any, error) {
				onRejectedCalled = true

				return nil, nil
			})

			deferred.Reject(dummyReason)

			assertEqual(t, onRejectedCalled, false)
		})

		testSync(t, "when `onRejected` is added immediately after the promise is rejected", func(t *testing.T) {
			deferred := deferred.New()
			dummyReason := createDummyReason()
			onRejectedCalled := false

			deferred.Reject(dummyReason)

			deferred.Promise.Catch(func(_ error) (any, error) {
				onRejectedCalled = true

				return nil, nil
			})

			assertEqual(t, onRejectedCalled, false)
		})

		testAsync(t, "when `onRejected` is added inside an `onFulfilled`", func(t *testing.T, done func()) {
			instance1 := promise.Resolve(nil)
			instance2 := promise.Reject(nil)
			firstOnFulfilledFinished := false

			instance1.Then(func(_ any) (any, error) {
				instance2.Catch(func(_ error) (any, error) {
					assertEqual(t, firstOnFulfilledFinished, true)
					done()

					return nil, nil
				})

				firstOnFulfilledFinished = true

				return nil, nil
			})
		})

		testAsync(t, "when one `onRejected` is added inside another `onRejected`", func(t *testing.T, done func()) {
			instance := promise.Reject(nil)
			firstOnRejectedFinished := false

			instance.Catch(func(_ error) (any, error) {
				instance.Catch(func(_ error) (any, error) {
					assertEqual(t, firstOnRejectedFinished, true)
					done()

					return nil, nil
				})

				firstOnRejectedFinished = true

				return nil, nil
			})
		})

		testAsync(t, "when the promise is rejected asynchronously", func(t *testing.T, done func()) {
			deferred := deferred.New()
			dummyReason := createDummyReason()
			firstStackFinished := false

			timeout(0, func() {
				deferred.Reject(dummyReason)
				firstStackFinished = true
			})

			deferred.Promise.Catch(func(_ error) (any, error) {
				assertEqual(t, firstStackFinished, true)
				done()

				return nil, nil
			})
		})
	})
}

func Test225(t *testing.T) {
	testPrepare(t)

	t.Skip("impossible because there is no global")
}

func Test2261(t *testing.T) {
	testPrepare(t)

	t.Skip("order of precedence is not guaranteed for performance reasons")
}

func Test2262(t *testing.T) {
	testPrepare(t)

	t.Skip("order of precedence is not guaranteed for performance reasons")
}

func Test2272(t *testing.T) {
	testPrepare(t)

	testFulfilledValue := createDummyValue()
	testFulfilled(t, testFulfilledValue, func(t *testing.T, instance *promise.Promise, done func()) {
		dummyReason := createDummyReason()

		next := instance.Then(func(_ any) (any, error) {
			return nil, dummyReason
		})

		next.Catch(func(reason error) (any, error) {
			assertEqual(t, reason, dummyReason)
			done()

			return nil, nil
		})
	})

	testRejectedValue := createDummyReason()
	testRejected(t, testRejectedValue, func(t *testing.T, instance *promise.Promise, done func()) {
		dummyReason := createDummyReason()

		next := instance.Catch(func(_ error) (any, error) {
			return nil, dummyReason
		})

		next.Catch(func(reason error) (any, error) {
			assertEqual(t, reason, dummyReason)
			done()

			return nil, nil
		})
	})
}

func Test2273(t *testing.T) {
	testPrepare(t)

	t.Skip("impossible because implementation is strictly typified")
}

func Test2274(t *testing.T) {
	testPrepare(t)

	t.Skip("impossible because implementation is strictly typified")
}

func Test231(t *testing.T) {
	testPrepare(t)

	testAsync(t, "via return from a fulfilled promise", func(t *testing.T, done func()) {
		dummyValue := createDummyValue()

		var instance *promise.Promise
		instance = promise.Resolve(dummyValue).Then(func(_ any) (any, error) {
			return instance, nil
		})

		instance.Catch(func(reason error) (any, error) {
			assertError(t, reason, "Chaining cycle detected for promise")
			done()

			return nil, nil
		})
	})

	testAsync(t, "via return from a rejected promise", func(t *testing.T, done func()) {
		dummyReason := createDummyReason()

		var instance *promise.Promise
		instance = promise.Reject(dummyReason).Catch(func(_ error) (any, error) {
			return instance, nil
		})

		instance.Catch(func(reason error) (any, error) {
			assertError(t, reason, "Chaining cycle detected for promise")
			done()

			return nil, nil
		})
	})
}
