FROM golang:latest as build-arm

RUN mkdir /app
WORKDIR /app
COPY ./ .
RUN GOOS=linux GOARCH=arm go build -a -installsuffix cgo -ldflags="-w -s" -o events

FROM golang:latest as build-arm64
RUN mkdir /app
WORKDIR /app
COPY ./ .
RUN GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags="-w -s" -o events

FROM golang:latest as build-amd64
RUN mkdir /app
WORKDIR /app
COPY ./ .
RUN GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o events



FROM scratch as arm
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/app /go/bin/app
COPY --from=build-arm /app/events /go/bin/events
ENTRYPOINT [ "/go/bin/events" ]

FROM scratch as arm64
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/app /go/bin/app
COPY --from=build-arm64 /app/events /go/bin/events
ENTRYPOINT ["/go/bin/events"]

FROM scratch as amd64
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/app /go/bin/app
COPY --from=build-amd64 /app/events /go/bin/events
ENTRYPOINT ["/go/bin/events"]