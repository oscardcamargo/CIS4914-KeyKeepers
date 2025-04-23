FROM golang:1.23

ENV CGO_ENABLED=1
WORKDIR /usr/src/app/src

COPY ./go.mod ./go.sum ./
COPY KeyKeeperClient.go/ ./
COPY message.pb.go/ ./
COPY message.proto/ ./
EXPOSE 8080-8100
RUN go mod download && go mod verify

RUN go build -v -o /usr/local/bin/app ./...

ENTRYPOINT ["app"]