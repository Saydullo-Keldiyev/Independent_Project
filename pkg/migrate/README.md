# pkg/migrate — Database Migration Framework

A lightweight, custom database migration framework for the auction-system with full rollback support.

## Features

- **Schema tracking**: `schema_migrations` table tracks applied versions and dirty state
- **Forward migrations** (`up`): Apply all pending migrations in version order
- **Rollback support** (`down`): Reverse migrations with explicit down operations
- **Timeout enforcement**: 300s for forward migrations, 120s for rollback (configurable)
- **Automatic rollback on failure**: When a migration fails, already-applied migrations in the current run are rolled back
- **Kubernetes Job integration**: Runs as a pre-deployment Job with proper security context
- **Dirty state detection**: Migrations that fail mid-execution are marked dirty for investigation

## Requirements Coverage

| Requirement | Description | Implementation |
|-------------|-------------|----------------|
| 7.1 | Run as K8s Job within 300s before app pods | `activeDeadlineSeconds: 300` in Job manifest |
| 7.2 | Every migration has reversible "down" operation | `Migration.Down` field is required |
| 7.5 | On failure: halt, rollback within 120s, report | Auto-rollback + `Result.FailedMigration` |

## Usage

### Define Migrations

```go
package migrations

import (
    "context"
    "github.com/auction-system/pkg/migrate"
    "github.com/jackc/pgx/v5/pgxpool"
)

var All = []migrate.Migration{
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
}
```

### Create a Migration Binary

```go
package main

import (
    "os"
    "github.com/auction-system/pkg/migrate"
    "your-service/internal/migrations"
)

func main() {
    migrate.RunCLI(migrations.All, os.Args[1:]...)
}
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | (required) |
| `MIGRATION_TIMEOUT` | Overall timeout in seconds | 300 |
| `ROLLBACK_TIMEOUT` | Rollback timeout in seconds | 120 |

### CLI Commands

```bash
# Apply all pending migrations
./migrate up

# Rollback last migration
./migrate down

# Rollback last 3 migrations
./migrate down 3

# Show migration status
./migrate status
```

### Kubernetes Deployment

Deploy the migration Job before app pods using ArgoCD sync waves or Helm hooks.
See `deployments/k8s/migrations/migration-job.yaml` for the Job template.

## Schema Migrations Table

```sql
CREATE TABLE schema_migrations (
    version     BIGINT PRIMARY KEY,
    dirty       BOOLEAN NOT NULL DEFAULT false,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
