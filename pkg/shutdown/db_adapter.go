package shutdown

// PgxPoolCloser wraps a pgxpool.Pool-like interface for graceful shutdown.
// This avoids importing pgx directly, keeping the shutdown package dependency-light.
// Any type that has a Close() method (like *pgxpool.Pool or *redis.Client) can be
// used directly as a DatabaseCloser.
//
// Usage:
//
//	pool, _ := pgxpool.New(ctx, connString)
//	handler := shutdown.New(cfg, shutdown.WithDatabase(shutdown.PgxPoolAdapter(pool)))
//
// Or directly, since *pgxpool.Pool already satisfies DatabaseCloser via its Close() method:
//
//	handler := shutdown.New(cfg, shutdown.WithDatabase(&pgxPoolWrapper{pool}))

// CloserFunc adapts a function to the DatabaseCloser interface.
// This is useful for wrapping any cleanup function as a database closer.
//
// Usage:
//
//	handler := shutdown.New(cfg,
//	    shutdown.WithDatabase(shutdown.CloserFunc(func() error {
//	        pool.Close()
//	        return nil
//	    })),
//	)
type CloserFunc func() error

// Close calls the underlying function.
func (f CloserFunc) Close() error {
	return f()
}

// NoOpCloser returns a DatabaseCloser that does nothing.
// Useful for testing or when a component has already been shut down.
func NoOpCloser() DatabaseCloser {
	return CloserFunc(func() error { return nil })
}
