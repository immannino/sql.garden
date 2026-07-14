# Connections

sql.garden can query live databases in addition to imported files. Connections are managed in **Settings → Connections**.

## Supported sources

| Source | Notes |
|--------|-------|
| PostgreSQL | Via DuckDB `postgres_scanner` extension |
| MySQL | Via DuckDB `mysql_scanner` extension |
| SQLite | `.sqlite` / `.db` files opened directly |
| DuckDB file | `.duckdb` files opened directly |

## Adding a connection

1. Open **Settings → Connections**
2. Click **Add Connection**
3. Choose the database type and enter connection details
4. Click **Test** to verify, then **Save**

Once saved, the connection appears in the sidebar. Click it to browse tables — each table opens as a data node on the canvas.

## S3 / object storage

S3 credentials are separate from database connections. Configure them in **Settings → S3 Storage**:

| Field | Description |
|-------|-------------|
| Endpoint | Leave blank for AWS S3; set for R2 (`https://<account>.r2.cloudflarestorage.com`) or MinIO |
| Key ID | AWS access key ID or equivalent |
| Secret | AWS secret access key or equivalent |
| Region | Required for AWS SigV4 signing (e.g. `us-east-1`) |

Once configured, you can import files via the `s3://` URI scheme or browse your buckets in the S3 panel.
