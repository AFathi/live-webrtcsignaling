#
# LOCAL
# 2 types of containers
#  builder : src code & go tools & libs & dependencies
#  release : resulting binary
builder:
	docker build --build-arg GITHUB_CREDENTIALS=tribeauto:d8349d4ecf06b353af5461e69666dabdebb41d52 --target builder -t live-webrtcsignaling:builder .
base:
	docker build --build-arg GITHUB_CREDENTIALS=tribeauto:d8349d4ecf06b353af5461e69666dabdebb41d52 --target base -t live-webrtcsignaling:base .
release:
	docker build --build-arg GITHUB_CREDENTIALS=tribeauto:d8349d4ecf06b353af5461e69666dabdebb41d52 --target release -t live-webrtcsignaling .


#
# GCE
#
pushReleaseToDev: base release
	@docker login -u _json_key -p $(GCE_JSON_KEY_DEV) https://gcr.io
	@docker tag live-webrtcsignaling gcr.io/tribe-api-dev/livewebrtcsignaling:latestmcu
	@docker push gcr.io/tribe-api-dev/livewebrtcsignaling:latestmcu

all: builder release

#
# add network delay (FIXME: should this be here ... ?)
#
addDelay:
	docker run -v /var/run/docker.sock:/var/run/docker.sock -ti pumba pumba --debug netem --duration 1m delay --time 2000 infradockercompose_live-webrtcsignaling_1

addLoss:
	docker run -v /var/run/docker.sock:/var/run/docker.sock -ti pumba pumba --debug netem --duration 1m loss --percent 5 --correlation 20 infradockercompose_live-webrtcsignaling_1

#
# go tools
# you will need to export your infra dockercompose directory
# export TRIBE_INFRA_DOCKERCOMPOSE=/home/marc/workspace/git/tribe/infra-dockercompose
#
go-get:
	docker run -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-unversioned:/vol-gopath-unversioned -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-versioned:/vol-gopath-versioned -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-versioned/src/github.com/heytribe/live-webrtcsignaling:/go/src/app -t infradockercompose_live-webrtcsignaling go get

go-build:
	docker run -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-unversioned:/vol-gopath-unversioned -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-versioned:/vol-gopath-versioned -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-versioned/src/github.com/heytribe/live-webrtcsignaling:/go/src/app -t infradockercompose_live-webrtcsignaling go build

go-vet:
	docker run -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-unversioned:/vol-gopath-unversioned -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-versioned:/vol-gopath-versioned -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-versioned/src/github.com/heytribe/live-webrtcsignaling:/go/src/app -t infradockercompose_live-webrtcsignaling go vet .

go-check-unused:
	docker run -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-unversioned:/vol-gopath-unversioned -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-versioned:/vol-gopath-versioned -v $(TRIBE_INFRA_DOCKERCOMPOSE)/vol-gopath-versioned/src/github.com/heytribe/live-webrtcsignaling:/go/src/app -t infradockercompose_live-webrtcsignaling bash -c 'go get honnef.co/go/tools/cmd/unused && /vol-gopath-unversioned/bin/unused'
