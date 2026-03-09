BINARY_NAME=recall

build:
	go build -o $(BINARY_NAME)

run:
	go run . $(ARGS)

test:
	go test ./...
	go test -tags=e2e ./...

test-unit:
	go test ./...

test-e2e:
	go test -tags=e2e ./...

clean:
	rm -f $(BINARY_NAME)
