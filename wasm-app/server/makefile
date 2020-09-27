build:
	@cd ../app && make build
	@cp ../app/app.wasm ./web
	@go build -o server

run: build
	./server

clean:
	@go clean -v ./...
	@-rm web/app.wasm
	@-rm ../app/app.wasm