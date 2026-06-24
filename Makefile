OS:=linux-amd64
GUA:= gua
JWTGENERATE:=jwt-generate
TAG := `git describe --abbrev=0 --tags`
TZ := Asia/Taipei
DATETIME := `TZ=$(TZ) date +%Y/%m/%d.%T`
show-tag:
	echo $(TAG)
buildgua = GOOS=$(1) GOARCH=$(2) go build -ldflags "-X main.version=$(TAG) -X main.name=$(GUA) -X main.compileDate=$(DATETIME)($(TZ))" -a -o build/$(GUA)$(3)
dockerbuildgua = CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) go build -ldflags "-X main.version=$(TAG) -X main.name=$(GUA) -X main.compileDate=$(DATETIME)($(TZ))" -a -o build/$(GUA)$(3)
tar = cp env.example ./build && cp -R testdata ./build/ && cd build/ && tar -zcvf $(GUA)_$(TAG)_$(1)_$(2).tar.gz $(GUA)$(3) env.example  testdata/ && rm $(GUA)$(3) env.example  && rm -rf testdata/

build/linux:
	go test ./...
	$(call buildgua,linux,amd64,)
	cp env.example build/ && cp -R testdata build/
dockerbuild/linux:
	go test ./...
	$(call dockerbuildgua,linux,amd64,)
build/linux_amd64.tar.gz: build/linux
	$(call tar,linux,amd64,)
build/darwin:
	go test ./...
	$(call buildgua,darwin,amd64,)
	cp env.example build/ && cp -R testdata build/
build/darwin_amd64.tar.gz: build/darwin
	$(call tar,darwin,amd64,)
clean:
	rm -rf build/
docker-build-GUA:
	go build -ldflags "-X main.name=$(GUA) -X main.version=$(TAG) " -a -o ./$(GUA);
