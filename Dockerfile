
FROM golang:1.2

RUN mkdir /intrinio

WORKDIR /intrinio

COPY . /intrinio

CMD /bin/bash