FROM golang as builder

COPY server.go go.mod ./

RUN CGO_ENABLED=0 go build

FROM alpine:3.20.3

COPY --from=builder /go/server /server
COPY index.html /

EXPOSE 8080

CMD ["/server"]
