package audit

import "context"

type Sink interface {
	Write(ctx context.Context, e Event) error
}
