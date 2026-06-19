package migrate_test

import (
	"context"
	"fmt"

	"github.com/auction-system/pkg/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ExampleMigration demonstrates how to define migrations for a service.
func Example() {
	migrations := []migrate.Migration{
		{
			Version: 20240101000001,
			Name:    "create_users_table",
			Up: func(ctx context.Context, pool *pgxpool.Pool) error {
				_, err := pool.Exec(ctx, `
					CREATE TABLE users (
						id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
						email VARCHAR(255) UNIQUE NOT NULL,
						created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
					);
				`)
				return err
			},
			Down: func(ctx context.Context, pool *pgxpool.Pool) error {
				_, err := pool.Exec(ctx, "DROP TABLE IF EXISTS users;")
				return err
			},
		},
		{
			Version: 20240101000002,
			Name:    "add_user_name_column",
			Up: func(ctx context.Context, pool *pgxpool.Pool) error {
				_, err := pool.Exec(ctx, "ALTER TABLE users ADD COLUMN name VARCHAR(255);")
				return err
			},
			Down: func(ctx context.Context, pool *pgxpool.Pool) error {
				_, err := pool.Exec(ctx, "ALTER TABLE users DROP COLUMN name;")
				return err
			},
		},
	}

	fmt.Printf("Registered %d migrations\n", len(migrations))
	// Output: Registered 2 migrations
}
