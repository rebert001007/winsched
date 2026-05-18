.PHONY: build clean test

build:
	go build -ldflags="-s -w" -trimpath -o winsched.exe .

clean:
	del /f winsched.exe winsched.test.exe 2>nul || true

test:
	go test ./...

run:
	go run . run .\config.yaml
