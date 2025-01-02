FROM golang:1-alpine AS builder

RUN apk add --no-cache ca-certificates
WORKDIR /build/css.gomuks.app
COPY . /build/css.gomuks.app
ENV CGO_ENABLED=0
RUN go build -o /usr/bin/css.gomuks.app

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/bin/css.gomuks.app /usr/bin/css.gomuks.app

ENTRYPOINT ["/usr/bin/css.gomuks.app"]
