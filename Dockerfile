FROM golang as builder

COPY server.go go.mod ./

RUN CGO_ENABLED=0 go build

FROM alpine

COPY --from=builder /go/server /server
COPY index.html /

EXPOSE 8080

CMD ["/server"]
