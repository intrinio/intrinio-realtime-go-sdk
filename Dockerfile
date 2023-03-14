FROM golang:1.20.1

RUN mkdir /intrinio

COPY . /intrinio

WORKDIR /intrinio/example

ENV INTRINIO_API_KEY=YOUR_API_KEY_HERE

RUN go get .

CMD go run .