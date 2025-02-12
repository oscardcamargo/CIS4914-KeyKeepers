FROM golang:1.23

ENV CGO_ENABLED=1
WORKDIR /usr/src/app/src

COPY src/go.mod src/go.sum ./
COPY src/ ./
COPY malware_hashes.db ../
RUN go mod download && go mod verify

RUN go build -v -o /usr/local/bin/app ./...

CMD ["app"]