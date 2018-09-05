#!/bin/sh

cd /build/livewebrtcsignaling
make
mkdir -p /go/app/usr/local/bin/livewebrtcsignaling
cp livewebrtcsignaling /go/app/usr/local/bin/livewebrtcsignaling
tar cf - /usr/local/* | tar xf - -C /go/app/
