package runtime

import "fmt"

func Chain(middleware []Middleware, final Next) Next {
	next := final
	for i := len(middleware) - 1; i >= 0; i-- {
		next = middleware[i](next)
	}
	return next
}

func RecoveryMiddleware(log Logger) Middleware {
	if log == nil {
		log = NopLogger{}
	}
	return func(next Next) Next {
		return func(ctx InvocationContext) (res Handled) {
			defer func() {
				if r := recover(); r != nil {
					log.Error(fmt.Sprintf("plugin-kit-ai: panic: %v", r))
					res = Handled{
						Err: fmt.Errorf("panic in hook handler: %v", r),
					}
				}
			}()
			return next(ctx)
		}
	}
}
