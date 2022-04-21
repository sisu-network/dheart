FROM golang:1.18-alpine as builder

ENV GO111MODULE=on

WORKDIR /tmp/go-app

RUN apk add --no-cache make gcc musl-dev linux-headers git

# # Though the id_rsa file is removed at the end of this docker build, it's still dangerous to include
# # id_rsa in the build file since docker build steps are cached. Only do this while our repos are in
# # private mode.

COPY go.mod .

COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o ./out/dheart main.go

# Start fresh from a smaller image
FROM alpine:3.9

WORKDIR /app

COPY --from=builder /tmp/go-app/out/dheart /app/dheart
COPY --from=builder /tmp/go-app/db/migrations /app/db/migrations

COPY .env.docker.local /app/.env

CMD ["./dheart"]
