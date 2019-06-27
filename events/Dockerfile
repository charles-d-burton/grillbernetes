FROM golang:alpine as build

RUN apk add --update --no-cache ca-certificates git gcc libc-dev
RUN mkdir /app
WORKDIR /app
COPY ./events/ .

ARG GOARCH=amd64
ENV GOARCH $GOARCH
RUN echo "Building for arch: $GOARCH"
RUN GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /go/bin/app

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/app /go/bin/app
COPY ./events/templates /go/bin/templates
ENTRYPOINT ["/go/bin/app"]