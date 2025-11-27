# Image upload API (Go + Cloudinary + MySQL)

This small Go API accepts multipart/form-data file uploads, uploads images to Cloudinary (using `CLOUDINARY_URL`), and stores the returned secure URL in a MySQL `images` table.

Environment variables
- `CLOUDINARY_URL` — the Cloudinary URL (example provided in `.env.example`).
- `MYSQL_DSN` — MySQL DSN, e.g. `user:pass@tcp(127.0.0.1:3306)/dbname?parseTime=true`.

Run locally

1. Populate environment variables, for example:

```bash
export CLOUDINARY_URL="cloudinary://839921722535737:f2qSYSOgl9uJTBYGmSCbWApISVc@dixhb99pp"
export MYSQL_DSN="root:pass@tcp(127.0.0.1:3306)/mydb?parseTime=true"
```

2. Fetch deps and build:

```bash
go mod tidy
go build -o upload-server
./upload-server
```

3. Upload an image (multipart form field name `file`):

```bash
curl -X POST -F "file=@./example.jpg" http://localhost:8080/upload
```

The server returns JSON: {"url":"https://..."} and the URL is stored in the `images` table.

Database migration

Run the SQL in `migration.sql` (or the server will create the table automatically).
