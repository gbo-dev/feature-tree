set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

default:
	@just --list

fmt:
	gofmt -w $(git ls-files '*.go')

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

build:
	go build -buildvcs=false -o ft ./cmd/ft

install dest="${HOME}/.local/bin/ft":
	mkdir -p "$(dirname "{{dest}}")"
	go build -buildvcs=false -o "{{dest}}" ./cmd/ft

ci: test race vet build

clean:
	rm -f ft coverage.out