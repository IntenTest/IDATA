# Database

The `migrations` directory is the source of truth for the MySQL schema.
Migration files are immutable after release and run once in filename order.

Apply a migration with a MySQL account that is allowed to change the schema:

```shell
mysql --host="$IDATA_DB_HOST" \
  --port="${IDATA_DB_PORT:-3306}" \
  --user="$IDATA_DB_USER" \
  --password \
  "$IDATA_DB_NAME" < app/database/migrations/0001_initial_schema.sql
```

Do not store database passwords, server addresses, or private certificates in
this directory.
