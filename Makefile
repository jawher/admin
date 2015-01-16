# OSX makefile
default: all
get-deps:
	go get github.com/docopt/docopt-go
	go get github.com/mitchellh/packer/common/uuid
	go get github.com/pki-io/pki.io/config
	go get github.com/xeipuuv/gojsonschema
	go get golang.org/x/crypto/pbkdf2
	go get golang.org/x/crypto/pbkdf2

all:
	go build pki.io.go helpers.go runAPI.go runCA.go  runCert.go  runEntity.go  runOrg.go runAdmin.go runCSR.go runClient.go  runInit.go runNode.go
install:
	install -m 0755 pki.io /usr/local/bin
test:
	export GOPATH=$(pwd)/../../
	bats bats
clean:
	rm pki.io
