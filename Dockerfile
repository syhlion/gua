# build stage
FROM golang:1.12.4-alpine3.9 AS builder
WORKDIR /app
RUN apk update && apk add git && apk add make
ENV GO111MODULE=on
RUN apk add --update gcc g++
RUN git clone https://github.com/syhlion/gua.git &&\
    cd gua &&\
    make dockerbuild/linux

# final stage
FROM scratch
WORKDIR /gua/
COPY --from=builder /app/gua/build/gua .
EXPOSE 8888 7777 6666

ENTRYPOINT ["./gua"]



