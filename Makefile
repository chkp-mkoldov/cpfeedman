
dev:
	go run cpfeedman.go

kick:
	./sendMsg.sh

build:
	CGO_ENABLED=0 go build -tags netgo -ldflags '-w -s -extldflags "-static"' cpfeedman.go