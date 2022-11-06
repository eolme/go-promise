package panics

import (
	"errors"
	"fmt"

	promise "github.com/eolme/go-promise/promise"
)

type FunctionWithPanic func() any

func wrapPanic(value any) error {
	if err, ok := value.(error); ok {
		return err
	}

	if str, ok := value.(string); ok {
		return errors.New(str)
	}

	return errors.New(fmt.Sprintf("panic handled with %p", &value))
}

func PromisifyPanic(fn FunctionWithPanic) *promise.Promise {
	return promise.New(func(resolve promise.PromiseResolve, reject promise.PromiseReject) {
		defer func() {
			if value := recover(); value != nil {
				reject(wrapPanic(value))
			}
		}()

		resolve(fn())
	})
}
