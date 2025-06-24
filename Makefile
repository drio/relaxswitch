PRJ=relaxswitch
SS=pitest
AUTH=-u $(MQTT_USER) -P $(MQTT_PASS)
TOPIC=shellies/shelly1l-test/relay/0

.PHONY: rsync lint tidy checks vuln
checks: lint vuln test

vuln:
	govulncheck ./...

vuln/verbose:
	govulncheck -show verbose ./...

lint:
	golangci-lint run

.PHONY:build
build: clean bin/$(PRJ).linux.arm64

bin/$(PRJ).%:
	GOOS=$(word 1, $(subst ., ,$*)) GOARCH=$(word 2, $(subst ., ,$*)) go build -o $@

mqtt/pub/%:
	mosquitto_pub $(AUTH) -t '$(TOPIC)' -m '$*' -h 192.168.8.180

mqtt/sub:
	mosquitto_sub $(AUTH) -t '$(TOPIC)' -v  -h 192.168.8.180 -F "%I %t %p"

.PHONY: rsync/watch
rsync/watch:
	@ls * utils/* | entr -c -s "make bin/$(PRJ).linux.arm64 rsync && notify '🚀' '$(PRJ) rsync done'"

rsync: build
	ssh $(SS) "mkdir -p services/$(PRJ)"
	rsync -avz -e ssh enigma.mp3 start .env bin/$(PRJ).linux.arm64 $(SS):services/$(PRJ)/
	rsync -avz -e ssh ~/dev/github.com/drio/services-bootstrap/services/start  $(SS):services/

bin:
	mkdir bin

test:
	go test -v *.go

test/watch:
	@ls *.go | entr -c -s 'go test -failfast -v ./*.go && notify "💚" || notify "🛑"'

pkg: go.mod 
	go mod tidy

tidy: go.mod
	go mod tidy

go.mod:
	go mod init github.com/drio/$(PRJ)

coverage/html:
	go test -v -cover -coverprofile=c.out
	go tool cover -html=c.out

clean:
	rm -f c.out bin/*
