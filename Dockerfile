FROM golang:1.12.1-alpine3.9

RUN apk update && apk add git && apk add make
ENV GO111MODULE=on
RUN apk add --update gcc g++
RUN git clone https://github.com/syhlion/gua.git /go/src/gua &&\
    cd /go/src/gua && \
    make build/linux

WORKDIR /go/src/gua

EXPOSE 8888 7777 6666

ENTRYPOINT ["./build/gua"]
