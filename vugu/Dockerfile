FROM golang:alpine as build

RUN apk add --update --no-cache ca-certificates git gcc libc-dev
RUN mkdir /app
WORKDIR /app
COPY ./ .

RUN echo "Building for wasm"
RUN go run dist.go

FROM httpd:alpine
COPY --from=build /app/dist/ /usr/local/apache2/htdocs/