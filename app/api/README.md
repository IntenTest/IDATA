# API

This directory owns server-side HTTP handlers, database configuration, and
repositories. The browser should eventually load data through versioned routes:

- `GET /api/v1/test-cases`
- `GET /api/v1/test-suites`
- `GET /api/v1/test-runs`

The current local demo still uses in-browser sample data. A MySQL driver has not
been added because the project does not yet have an approved database package.
When one is selected, isolate it in a connection factory passed to
`app.api.database.Database`.

## Database environment

Production and local credentials must be supplied outside Git:

```text
IDATA_DB_HOST
IDATA_DB_PORT=3306
IDATA_DB_NAME
IDATA_DB_USER
IDATA_DB_PASSWORD
IDATA_DB_CONNECT_TIMEOUT=10
IDATA_DB_SSL_CA
```

`IDATA_DB_SSL_CA` is optional in code, but remote production connections
should use TLS when the MySQL server supports it.
