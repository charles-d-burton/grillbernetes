FROM golang:latest as build-arm

RUN mkdir /app
WORKDIR /app
COPY ./ .
RUN GOOS=linux GOARCH=arm go build -a -installsuffix cgo -ldflags="-w -s" -o control-hub

FROM golang:latest as build-arm64
RUN mkdir /app
WORKDIR /app
COPY ./ .
RUN GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags="-w -s" -o control-hub

FROM golang:latest as build-amd64
RUN mkdir /app
WORKDIR /app
COPY ./ .
RUN GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o control-hub

FROM scratch as arm
COPY --from=build-arm /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-arm /app/control-hub /go/bin/control-hub
ENTRYPOINT [ "/go/bin/control-hub" ]

FROM scratch as arm64
COPY --from=build-arm64 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-arm64 /app/control-hub /go/bin/control-hub
ENTRYPOINT ["/go/bin/control-hub"]

FROM scratch as amd64
COPY --from=build-amd64 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-amd64 /app/control-hub /go/bin/control-hub
ENTRYPOINT ["/go/bin/control-hub"]