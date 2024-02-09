package ctxWithCancel

import "context"

type ContextWithCancel struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}
