BINARY_NAME=recall

build:
	go build -o $(BINARY_NAME)

run:
	go run . $(ARGS)

test:
	go test ./...

clean:
	rm -f $(BINARY_NAME)
