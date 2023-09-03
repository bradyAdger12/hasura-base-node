# Hasura Base

This is a starter project that uses TODO.

For a node.js based version see : [https://github.com/aaronblondeau/hasura-base](https://github.com/aaronblondeau/hasura-base)

Authentication is handled by Hasura and JWTs.

## Getting Started

1.  Install dependencies

```
go get
```

TODO : https://hasura.io/docs/latest/hasura-cli/install-hasura-cli/

2. Copy .env.example to .env and update values (note "TODO" values).  The S3 secret and key will be updated in step 4 below.

3. Start the docker containers

Before starting the containers, switch the postgres image to something like "postgis/postgis:15-3.3" in docker-compose.yml if you need PostGIS support.

```
docker compose up -d
```

4. Use the minio UI (http://localhost:9090/) to create a 'user-public' bucket as well as to create an api access key and secret. Update S3_ACCESS_KEY and S3_SECRET_KEY in .env file.

5. Start the golang server

```
go run main.go
```

6. Run hasura migrations and apply metadata

```
setx HASURA_GRAPHQL_ADMIN_SECRET "mydevsecret"
```

```
hasura migrate apply --project ./hasura
hasura metadata apply --project ./hasura
```

7. Start the hasura console

```
hasura console --project ./hasura
```

8. Use the hasura console to create additonal tables, actions, events, relationships, and permissions.

Other admin tools are available at (see .env file for passwords):

Minio UI : http://localhost:9090/

9. To update prisma schema after hasura db updates:

```
go run github.com/steebchen/prisma-client-go db pull
go run github.com/steebchen/prisma-client-go generate
```

10. When done, stop the docker containers

```
docker compose down
```

## Troubleshooting

https://goprisma.org/docs/getting-started/quickstart

## TODO

How to set go prisma db conn string from env var?