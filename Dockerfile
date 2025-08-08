FROM golang:1.24.0

RUN mkdir /intrinio

COPY . /intrinio

WORKDIR /intrinio/example

ENV INTRINIO_API_KEY=YOUR_API_KEY_HERE

RUN go get .

CMD go run .