package xroute

import "context"

func ctxIsTrue(ctx context.Context, key interface{}) (v bool) {
	if vi := ctx.Value(key); vi != nil {
		v = vi.(bool)
	}
	return
}