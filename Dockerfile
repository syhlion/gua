# build stage
FROM golang:1.12.2-alpine3.9 AS builder
RUN apk update && apk add git && apk add make
ENV GO111MODULE=on
RUN apk add --update gcc g++
RUN git clone https://github.com/syhlion/gua.git /go/src/gua &&\
    cd /go/src/gua && \
    make build/linux


# final stage
FROM scratch
COPY --from=builder /go/src/gua/build/gua /
EXPOSE 8888 7777 6666

ENTRYPOINT ["./gua"]



