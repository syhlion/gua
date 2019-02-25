OS:=linux-amd64
GUA:= gua
GUA-NODE:= gua-node
JWTGENERATE:=jwt-generate
TAG := `git describe --abbrev=0 --tags`
TZ := Asia/Taipei
DATETIME := `TZ=$(TZ) date +%Y/%m/%d.%T`
show-tag:
	echo $(TAG)
buildgua = GOOS=$(1) GOARCH=$(2) go build -ldflags "-X main.version=$(TAG) -X main.name=$(GUA) -X main.compileDate=$(DATETIME)($(TZ))" -a -o build/$(GUA)$(3) 
buildguanode = GOOS=$(1) GOARCH=$(2) go build -ldflags "-X main.version=$(TAG) -X main.name=$(GUA-NODE) -X main.compileDate=$(DATETIME)($(TZ))" -a -o build/node/$(GUA-NODE)$(3)  ./node/
tar = cp env.example ./build && cp node/env.example ./build/node && cp -R testdata ./build/ && cd build/ && tar -zcvf $(GUA)_$(TAG)_$(1)_$(2).tar.gz node/ $(GUA)$(3) env.example  testdata/ && rm $(GUA)$(3) env.example  && rm -rf testdata/ && rm -rf node/

build/linux: 
	go test
	$(call buildgua,linux,amd64,)
	$(call buildguanode,linux,amd64,)
	cp env.example build/ &&  cp node/env.example build/node/ && cp -R testdata build/
build/linux_amd64.tar.gz: build/linux
	$(call tar,linux,amd64,)
build/windows: 
	go test
	$(call buildgua,windows,amd64,.exe)
	$(call buildguanode,windows,amd64,.exe)
	cp env.example build/ &&  cp node/env.example build/node/ && cp -R testdata build/
build/windows_amd64.tar.gz: build/windows
	$(call tar,windows,amd64,.exe)
build/darwin: 
	go test
	$(call buildgua,darwin,amd64,)
	$(call buildguanode,darwin,amd64,)
	cp env.example build/ &&  cp node/env.example build/node/ && cp -R testdata build/
build/darwin_amd64.tar.gz: build/darwin
	$(call tar,darwin,amd64,)
clean:
	rm -rf build/
docker-build-GUA:
	go build -ldflags "-X main.name=$(GUA) -X main.version=$(TAG) " -a -o ./$(GUA);
docker-build-GUA-NODE:
	go build -ldflags "-X main.name=$(GUA-NODE) -X main.version=$(TAG) " -a -o ./$(GUA-NODE);
