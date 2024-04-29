FROM golang:alpine as builder
COPY main.go .
RUN CGO_ENABLED=0 go build -o /go/bin/peephole main.go

FROM scratch
COPY --from=builder /go/bin/peephole /peephole
ENTRYPOINT ["/peephole"]