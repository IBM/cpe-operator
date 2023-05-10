FROM golang:1.16

ENV GOOS=linux
ENV GOARCH=amd64

ENV PATH="$PATH:/go/bin/linux_amd64:/go/bin"

WORKDIR /go/src/cpe-parser
COPY parser parser
COPY common common
COPY main.go main.go
COPY api.go api.go
COPY go.mod go.mod
RUN go mod tidy
RUN go get -d -v ./...
RUN go install -v ./...

CMD ["cpe-parser"]