# go-promise

> A promise represents the eventual result of an asynchronous operation

This implementation does not seek to comply with [the specification](https://promisesaplus.com/),
however, aims to be at least compatible but also faster and more asynchronous.

For example, the order of precedence is not guaranteed so as not to block anything unnecessary.

### Usage

```go
prom := promise.New(func (resolve promise.PromiseResolve, reject promise.PromiseReject) {
  // note: synchronous as the top-level context
  // ... do something ...
  if (ok) {
    resolve(value)
  } else {
    reject(err)
  }
})

// ... do something ...

prom.Then(func (result any) (any, error) {
  // note: asynchronous context
  log.Print(result) // value

  return nil, nil
})
```

### Async-await

You can also use `Async`/`Await` abstractions:

```go
prom := async.Async(func () (any, error) {
  // note: asynchronous context
  return "hello from 2077"
})

// ... do something ...

result, _ := async.Await(prom)
log.Print(result) // hello from 2077
```

### Deferred

The creation of Promise is synchronous, so you can use the `Deferred` abstraction,
which exposes `Resolve`/`Reject` functions:

```go

def := deferred.New()

def.Then(func (result any) (any, error) {
  // note: asynchronous context
  log.Print(result) // wow
})

def.Resolve("wow")

```

### Handling panics

If you need to handle panics, use `PromisifyPanic` like this:

```go
prom := panics.PromisifyPanic(func () {
  // note: asynchronous context
  // ... do something ...
  panic("something went wrong")
})

// ... do something ...

prom.Catch(func (reason error) (any, error) {
  // note: asynchronous context
  log.Print(reason) // something went wrong

  return nil, nil
})
```

## Installation

```shell
go get -u github.com/eolme/go-promise
```

## License

github.com/eolme/go-promise is [MIT licensed](./LICENSE).
