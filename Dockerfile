# build stage
FROM golang:1.19.2-alpine3.16 AS builder
LABEL stage=gua-intermediate
ENV GO111MODULE=on
RUN apk update && apk add git && apk add make
RUN apk add --update gcc g++
ADD ./ /go/src/gua
RUN cd /go/src/gua && go build -mod vendor

# final stage
FROM alpine:3.16.2
COPY --from=builder /go/src/gua/gua ./
EXPOSE 8888 7777 6666

ENTRYPOINT ["./gua"]
