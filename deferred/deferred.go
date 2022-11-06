package deferred

import promise "github.com/eolme/go-promise/promise"

type Deferred struct {
	Promise *promise.Promise
	Resolve promise.PromiseResolve
	Reject  promise.PromiseReject
}

func New() (deferred *Deferred) {
	deferred = &Deferred{
		Promise: nil,
		Resolve: nil,
		Reject:  nil,
	}

	deferred.Promise = promise.New(func(resolve promise.PromiseResolve, reject promise.PromiseReject) {
		deferred.Resolve = resolve
		deferred.Reject = reject
	})

	return deferred
}

func (self *Deferred) Then(then promise.PromiseThen) *promise.Promise {
	return self.Promise.Then(then)
}

func (self *Deferred) Catch(catch promise.PromiseCatch) *promise.Promise {
	return self.Promise.Catch(catch)
}

func (self *Deferred) ThenCatch(then promise.PromiseThen, catch promise.PromiseCatch) *promise.Promise {
	return self.Promise.ThenCatch(then, catch)
}

func (self *Deferred) Finally(finally promise.PromiseFinally) *promise.Promise {
	return self.Promise.Finally(finally)
}

func (self *Deferred) Wait() (unpacked any, reason error) {
	return self.Promise.Wait()
}
