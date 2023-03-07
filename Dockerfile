FROM golang:1.20.1

RUN mkdir /intrinio

COPY . /intrinio

WORKDIR /intrinio/example

RUN go get .

CMD go run .