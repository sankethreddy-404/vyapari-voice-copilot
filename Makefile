.PHONY: run test vet build clean

run:
	go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -trimpath -o bin/vyapari ./cmd/server

clean:
	rm -rf bin data/store.json
