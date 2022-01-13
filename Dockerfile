# build stage
FROM golang:1.16.6-alpine3.14 AS builder
LABEL stage=gua-intermediate
ENV GO111MODULE=on
RUN apk update && apk add git && apk add make
RUN apk add --update gcc g++
ADD ./ /go/src/gua
RUN cd /go/src/gua && go build -mod vendor

# final stage
FROM alpine:3.15.0
COPY --from=builder /go/src/gua/gua ./
EXPOSE 8888 7777 6666

ENTRYPOINT ["./gua"]