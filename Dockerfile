# build stage
FROM golang:1.25-alpine AS builder
LABEL stage=gua-intermediate
ENV CGO_ENABLED=0
RUN apk add --no-cache git make
ADD ./ /go/src/gua
RUN cd /go/src/gua && go build -mod vendor -o /gua

# final stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /gua /gua
EXPOSE 7777 6666
ENTRYPOINT ["/gua"]
