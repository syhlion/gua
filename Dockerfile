# build stage
FROM golang:1.12.2-alpine3.9 AS builder
WORKDIR /go/src/gua/build
RUN apk update && apk add git && apk add make
ENV GO111MODULE=on
RUN apk add --update gcc g++
RUN git clone https://github.com/syhlion/gua.git /go/src/gua &&\
    cd /go/src/gua && \
    make build/linux

# final stage
FROM scratch
WORKDIR /gua
COPY --from=builder ./gua /gua/
EXPOSE 8888 7777 6666

ENTRYPOINT ["./gua"]



