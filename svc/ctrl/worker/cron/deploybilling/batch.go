package deploybilling

import (
	"fmt"

	restate "github.com/restatedev/sdk-go"
)

// batchConcurrency caps parallel Restate RunAsync steps per batch so a fleet
// sweep does not hammer provider rate limits.
const batchConcurrency = 16

// runBatched fans out items in fixed-size batches of journaled RunAsync steps.
// step errors wedge the caller; expected per-item failures belong in R and are
// handled in collect.
func runBatched[T, R any](
	ctx restate.ObjectContext,
	items []T,
	name func(T) string,
	step func(restate.RunContext, T) (R, error),
	collect func(T, R),
) error {
	for start := 0; start < len(items); start += batchConcurrency {
		end := min(start+batchConcurrency, len(items))
		batch := items[start:end]

		futures := make([]restate.RunAsyncFuture[R], len(batch))
		for i, item := range batch {
			futures[i] = restate.RunAsync(ctx, func(rc restate.RunContext) (R, error) {
				return step(rc, item)
			}, restate.WithName(name(item)))
		}

		for i, fut := range futures {
			result, err := fut.Result()
			if err != nil {
				return fmt.Errorf("%s: %w", name(batch[i]), err)
			}
			collect(batch[i], result)
		}
	}
	return nil
}
