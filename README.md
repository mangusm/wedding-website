# wedding-website

[here](https://www.manguswedding.com)

## Development Setup

1. install [go](https://go.dev/doc/install), [dbmate](https://github.com/amacneil/dbmate/releases), and [air](https://github.com/cosmtrek/air/releases)

1. Create a `.env` file:
```
cat << EOF > .env
DATABASE_URL=postgres://postgres:asdf@127.0.0.1:5432/wedding?sslmode=disable
PORT=8080
ENVIRONMENT=dev
EOF
```

1. `docker compose up`

1. `dbmate up`

1. `air`