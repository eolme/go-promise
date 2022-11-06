package async

import promise "github.com/eolme/go-promise/promise"

type AsyncFunction func() (any, error)

func Async(fn AsyncFunction) *promise.Promise {
	return promise.New(func(resolve promise.PromiseResolve, reject promise.PromiseReject) {
		go func() {
			packed, reason := fn()

			if reason != nil {
				reject(reason)
			} else {
				resolve(packed)
			}
		}()
	})
}

func Await(promise *promise.Promise) (any, error) {
	return promise.Wait()
}
