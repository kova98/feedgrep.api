# API

### Migrations

The project uses [goose](https://github.com/pressly/goose) for migrations

```bash
# 1. Install goose
go install github.com/pressly/goose/v3/cmd/goose@latest

# 2. Create a new migration
goose create add_some_column sql

# 3. Set the DB_URL inside the Makefile 

# 4. Apply migrations
make migrate
```

Running `make migrate` will write the current schema into the `/data/schema/schema.sql` file.
