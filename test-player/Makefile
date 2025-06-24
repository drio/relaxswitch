.PHONY: build deploy clean

BINARY_NAME=test-player
GOKRAZY_INSTANCE=gktest

build:
	GOOS=linux GOARCH=arm64 go build -o $(BINARY_NAME) main.go

deploy: build
	scp $(BINARY_NAME) ../enigma.mp3 $(GOKRAZY_INSTANCE):/tmp
	ssh $(GOKRAZY_INSTANCE) 'chmod 755 /tmp/$(BINARY_NAME)'

clean:
	rm -f $(BINARY_NAME)

breakglass:
	breakglass $(GOKRAZY_INSTANCE)

all: deploy
