# wedding-website

[here](https://www.manguswedding.com)

## Development Setup

1. install [go](https://go.dev/doc/install), [docker](https://docs.docker.com/engine/install/) and [air](https://github.com/cosmtrek/air/releases) (optional)

1. Create a `.env` file:
```
cat << EOF > .env
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=wedding
POSTGRES_USER=wedding_user
POSTGRES_PASSWORD=
ENVIRONMENT=dev
PORT=8080
AWS_ACCOUNT_ID=
AWS_REGION=
ECR_REPO=
ECS_CLUSTER_NAME=
ECS_SERVICE_NAME=
EOF
```

1. `docker compose up`

1. `air` or `go run .`
