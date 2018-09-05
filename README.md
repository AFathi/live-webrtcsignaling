# webrtcsignaling

This project has been created to manage the WebRTC MCU  

Peers need a signaling server to exchange some informations like SDP / Candidate Exchange before initializing a P2P / STUN / TURN connection. It's written in GoLang.

# Coding styles & philosophy

we embrace the overly defensive programming paradigm  
we treat "preventable errors" (developers errors) like nil pointer / wrong library calls / ... as full errors.  

we try to have minimalistic dependencies requirements, to keep control & lower the exec footprint.  

the coding style respects goimports (standard gofmt variant)  

context ctx:  
 - the context ctx should flow through the program  
 - the context ctx should "inform", not "control". A function receiving the context should be able to read it's content to alter it's behavior, but you shouldn't pass context param as function parameters to control the function behavior.  

Object initialization:  
 - every "objects" should have builder func : New<ObjectName> ; this func should init maps & other fields.
 - if the object is used with parameters (external maps, ...), you should implement another New<ObjectName>With<WhateverParam> func

# Build

we use a Dockerfile to build the app  
the build is multi-stage : builder & release  
builder <=> contains a debian image with the dev tools, go source, libraries sources, packages sources, ...  
base    <=> contains a debian image with a copy of "builder" image libraries
release <=> base + a copy of "builder" image go live-webrtcsignaling binary

For dev env, you should use https://github.com/heytribe/infra-dockercompose  

## Dev using infra-dockercompose

```
git clone git@github.com:heytribe/infra-dockercompose.git
git submodule update --init --recursive
docker-compose build live-webrtcsignaling
docker-compose up live-webrtcsignaling rabbitmq
```

open your favorite IDE in infra-dockercompose/vol-gopath-versioned/src/github.com/heytribe/live-webrtcsignaling

## Dev inspection

you can also build the image outside docker compose, using the Makefile  

create the builder image, containing source code, dev tools, libraries sources, packages, ...
```
make builder
```

to create the base image
```
make base
```

to create a release image
```
make release
```

push the release image to GCE dev
```
make pushReleaseToDev
```

you can launch the image or spawn a shell against the image using run.sh script
```
./run.sh builder # launch the mcu:builder
./run.sh         # launch the mcu:release
./run.sh builder bash # launch a bash shell inside mcu:builder
./run.sh bash         # launch a bash shell inside mcu:release
```

## GCE dev
create release image
```
make release
```

push the release image to GCE dev
```
make pushReleaseToDev
```

restart service
```
/data/coreos/stopunitgroupmcu.sh beta
/data/coreos/startunitgroupmcu.sh beta
```

follow what's happening
```
fleetctl journal -follow -lines=1000 livewebrtcsignalingmcu@beta.service
```

## Network testing on your local env

install pumba
```
git clone git@github.com:gaia-adm/pumba.git && cd pumba && docker build -t pumba -f Dockerfile .
```
play with pumba netem (wrapper around linux Trafic Control: tc) to add 2sec delay
```
docker run -v /var/run/docker.sock:/var/run/docker.sock -ti pumba pumba --debug netem --duration 1m delay --time 2000 infradockercompose_live-webrtcsignaling_1
```

# Architecture

## Definition

Rooms: group of Room  
Room: single conversation room composed of "websocket connections objects"
Hub: simple index object map of websocketId => "websocket connection object"
FIXME

## High level overview

simplest mode: 2 peers communicating  

first peer connects  
```
[==========================================>]
[----- Publisher ----] [----- Listener -----]
Client 1 <---------> MCU <---------> Client 2
```

[==========================================>]
[----- Publisher ----] [----- Listener -----]
Client 1 <---------> MCU <---------> Client 2

// opening websocket
webrtcsignaling main() func is :
 - loading config
 - initializing global objects
 - start listening on http path "/" and "/api"

when client X is hitting "/api" using websocket => open WebSocket WS.
 - register WS in hub object.
 - start goroutines readPump & writePump on the WS.

 WS.readPump will pop messages on the websocket and send them to  connection.processMessage() -> connection.handleApi()

// signaling part
// client & mcu will exchange their SDP using the websocket
exchangeSdp()
 - if client is client 1 <=> publisher side:
     - parsing SDP given by client 1
     - create a webRTCSession  
     - answer SDP
     - create a stunContext    
     - start webRTCSession:  webRTCSession.serveWebRTC()
 - if client is client 2 <=> listener side:
     start webRTCSession:  webRTCSession.serveWebRTC()

// connection d'un peer aux autres
webRTCSession.connectListeners()
  si on est "client 1", pour tous les peers de la room autre que sois meme
    on cree un SDP (sdpCtx.createSdpOffer())
    on echange un SDP
    on crée une NewWebRTCSession


// serveWebRTC
  - ouvre socket UDP
  - start goroutines readPump & writePump sur la socket UDP.
     readPump call connudp.processPacket()
       -> si packet stun => stun.handleStunMessage() => ChState
  - lance stateManager
    - si un packet passe le ctx stun en StunStateCompleted
       - session webRTCSession est publisher
          - create GST decoder
          - create dtls session (dtlsclientconnect)
          - create srtp session
          - => event webRTCSession is UP
       - session webRTCSession est listener
          - create dtls session (dtlsServerAccept)
          - create GST encoder
          - create srtp session
          - => event webRTCSession is UP

TODO:
 - rename connection object into ws
 - remove webRTCSession create & stunContext create from exchangeSdp()



FIXME  
Just type 'make'. It will build a linux image ready to be dockerized.
'make docker' push the image on the tribe.pm private registry on CoreOS cluster

# Analysis

## Profiling

http://www.integralist.co.uk/posts/profiling-go/

### Memory profiling

@see /memstats http func handler in webrtcsignaling.go

### Trace

start a trace in the code :

```go
f, err = os.Create(time.Now().Format("trace.pprof"))
if err != nil {
  panic(err)
}
if err := trace.Start(f); err != nil {
  panic(err)
}
```

terminate the trace

```go
trace.Stop()
f.Close()
```

full example

```go
var f *os.File

f, err = os.Create(time.Now().Format("trace.pprof"))
if err != nil {
  panic(err)
}
if err := trace.Start(f); err != nil {
  panic(err)
}

http.HandleFunc("/tracestop", func (w http.ResponseWriter, r *http.Request) {
	trace.Stop()
	f.Close()
})
```

launch the trace analysis :

```sh
go tool trace tmp/runner-build trace.pprof
```

##

# How it works
`webrtcsignaling` open a secured websocket service with tribe.pm wildcard.
All the exchange negociation will be made with this API Websocket.
A hearthbeat is sent via a ping to test the link.


## Server message formats

### RPC Methods
An RPC method can be call sending a json message beginning with
```json
{
  "a"        :  "<action name>",
  "d"        :  "<data>
}
```

All the RPC methods return a json block beginning with
```json
{
  "a"  :  "<action name>R",
  "s"  :  true,
  "d"  :  {
                ...
          }
}
```
in case of success, where:

"a" means action (string with a R concatenated at the end)

"s" means success (boolean)

"d" is the data associated, if any returned

In case of errors, the RPC methods return a json block beginning with
```json
{
  "a" : "<action name>R",
  "s" : false,
  "e" : <error code number>
}
```

"a" mean action (string with R concatened at the end)

"s" mean success (boolean)

"e" is an error code encountered (integer), described in the section errors

### Events
A received event is represented as a json block beginning with
```json
{
  "a"        :  "<event name>",
  "d"        :  "<data>
}
```

## Server
Methods:

RPC:
- 1. join
- 2. reconnect
- 3. exchangeCandidate
- 4. exchangeSdp
- 5. orientationChange

Events:

- 6. eventExchangeCandidate
- 7. eventExchangeSdp
- 8. eventUserMediaConfiguration
- 9. eventLeave
- 10. eventOrientationChange
- 11. eventFreeze
- 12. eventCpu

### RPC calls

### 1. 'join'
***

### Description
***
Request for joining a roomId

### Syntax
***

#### *Request*
```json
{
  "a"        :  "join",
  "d"        : {
                 "roomId"      :  "<roomId>",
                 "bearer"      :  "<bearer>",
                 "platform"    :  "<platform>",
                 "deviceName"  :  "<deviceName>",
                 "networkType" :  "<networkType>",
                 "version"     :  "<version>",
                 "appVersion"  :  "<appVersion>",
                 "orientation" :  <orientation>,
                 "camera"      :  "<camera>"
               }
}
```
Name | Type | Description
---- | ---- | ----
roomId | String | Room Identifier
bearer | String | OAuth Bearer Token
platform | String | Platform used to join ("Android","iOS","Web")
deviceName | String | Device name (eg: iPhone 7, OnePlus 3 etc...)
networkType | String | Network type (two values: "wifi" or "mobile")
version | String | Version of the OS (eg: iOS 10.0.3 etc...) or browser (eg: chrome v57)
appVersion | String | Application version (tribe version. eg: 110)
orientation | Integer | Initial orientation of the device in degrees (0 == titled left, 90 == normal, 180 == titled right, 270 == upside down)
camera | String | Orientation of the camera ("front" or "back")

#### *Answer*
```json
{
  "a"   : "joinR",
  "s"   :  true,
  "d"   :  {
             "socketId" :  "<socketId>",
             "roomSize" :  <roomSize>,
             "userMediaConfiguration": {
               <userMediaConfiguration>
             }
             "sessions" :  [
               {
                 "socketId": "<socketId>",
                 "userId"  : "<userId>"
               },
               ...
             ]
           }
}
```
Name | Type | Description
---- | ---- | ----
socketId | String | Socket Identifier (32 bytes in hexadecimal string) of your publisher/connection used for reconnect
roomSize | Integer | Number of peers connected to the room (updated)
userMediaConfiguration | JSON Object | complete JSON configuration object for getUserMedia(<userMediaConfiguration>) eg: { "audio": true, "video": { "width": { "max": "<width>" }, "height": { "max": "<height>" }, "frameRate": { "min": "<fps>"}}}
sessions | Object Array | an array of sessions attached to the roomId
socketId | String | Socket Identifier (32 bytes in hexadecimal string)
userId | String | backend userId (short id) associated to the socketId

### 2. 'reconnect'
***

### Description
***
Request for reconnecting an existing socketId

### Syntax
***

#### *Request*
```json
{
  "a"        :  "reconnect",
  "d"        : {
                 "socketId"      :  "<socketId>"
               }
}
```
Name | Type | Description
---- | ---- | ----
socketId | String | Socket Identifier

#### *Answer*
```json
{
  "a"   : "reconnectR",
  "s"   :  true
}
```
Name | Type | Description
---- | ---- | ----
roomSize | Integer | Number of peers connected to the room (updated)

### 3. 'exchangeCandidate'
***

### Description
***
Request for sending an ICE candidate

### Syntax
***

#### *Request*
```json
{
  "a"         :  "exchangeCandidate",
  "d"         : {
                  "to"        :  "<socketId>",
                  "candidate" :  "<jsonICECandidate>"
                }
}
```
Name | Type | Description
---- | ---- | ----
socketId | String | Socket Identifier where the ICE candidate must be sent
jsonICECandidate | Json ICE candidate | WebRTC ICE candidate (eg: {"candidate":"candidate:1321500371 2 udp 1685987070 78.201.204.97 35188 typ srflx raddr 192.168.0.22 rport 35188 generation 0 ufrag fhni network-id 2 network-cost 10","sdpMid":"audio","sdpMLineIndex":0})

#### *Answer*
```json
{
  "a" :  "exchangeCandidateR",
  "s" :  true
}
```

### 4. 'exchangeSdp'
***

### Description
***
Request for sending a SDP (Session Description)

### Syntax
***

#### *Request*
```json
{
  "a"    :  "exchangeSdp",
  "d"    :  {
              "to"   :  "<socketId>",
              "sdp"  :  "<jsonSdp>"
            }
}
```
Name | Type | Description
---- | ---- | ----
socketId | String | Socket Identifier where the SDP must be sent
jsonSdp | JSON SDP | WebRTC Session description

#### *Answer*
```json
{
  "a" :  "exchangeSdpR",
  "s" :  true
}
```

### 5. 'orientationChange'
***

### Description
***
Request for sending an orientation mobile change. This request will generate a broadcast eventOrientationChange message to all peers connected on the same roomId

### Syntax
***

#### *Request*
```json
{
  "a"    :  "orientationChange",
  "d"    :  {
              "orientation"  :  <orientation>,
              "camera"       :  "<camera>"
            }
}
```
Name | Type | Description
---- | ---- | ----
orientation | Integer | New orientation of the device (0, 90, 180, 270)
camera | String | Orientation of the camera ("front" or "back")

#### *Answer*
```json
{
  "a" :  "orientationChangeR",
  "s" :  true
}
```

### Events calls

### 6. 'eventExchangeCandidate'
***

### Description
***
This is an event to receive an ICE Candidate from another peer on the joined RoomId

### Syntax
***

#### *Event*
```json
{
  "a"         :  "eventExchangeCandidate",
  "d"         :  {
                   "from"      :  {
                     "socketId": "<socketId>",
                     "userId"  : "<userId>"
                   },
                   "candidate" :  "<jsonICECandidate>"
                 }
}
```
Name | Type | Description
---- | ---- | ----
from | JSON Object | From which peer event came
socketId | String | Socket Identifier (32 bytes in hexadecimal string)
userId | String | backend userId (short id) associated to the socketId
candidate | Json ICE candidate | WebRTC ICE candidate

### 7. 'eventExchangeSdp'
***

### Description
***
This is an event to receive a SDP (Session description) from another peer on the joined RoomId

### Syntax
***

#### *Event*
```json
{
  "a"     :  "eventExchangeSdp",
  "d"     :  {
               "from"      :  {
                 "socketId": "<socketId>",
                 "userId"  : "<userId>"
               },
               "sdp"   :  "<jsonSdp>"
             }
}
```
Name | Type | Description
---- | ---- | ----
from | JSON Object | From which peer event came
socketId | String | Socket Identifier (32 bytes in hexadecimal string)
userId | String | backend userId (short id) associated to the socketId
jsonSdp | JSON SDP | WebRTC Session description

### 8. 'eventUserMediaConfiguration'
***

### Description
***
Event sent when new audio/video configuration changes (e.g, when a user joins or leaves the room).
You should set the new audio/video constraints detailed in the userMediaConfiguration field with getUserMedia() then removeStream / addStream the new stream returned by getUserMedia on all peerConnections with RTCPeerConnection.removeStream() and RTCPeerConnection.addStream(). Then an onnegotiationneeded event will be trigered.

Be careful, because you should call again a createOffer after replacing streams for all RTCPeerConnection answered with createOffer. The onnegotiationneeded call check if this is an offer or not (isOffer on the web poc) and call createOffer() only in this case. You should only call createOffer() manually after removeStream() / addStream() if the RTCPeerConnection has answered with a createAnswer() at init. Other RTCPeerConnection will call it through the method onnegotiationneeded() theorically, it depends of your implementation.

### Syntax
***

#### *Event*
```json
{
  "a"  :  "eventUserMediaConfiguration",
  "d"  :  <userMediaConfiguration>
}
```
Name | Type | Description
---- | ---- | ----
userMediaConfiguration | JSON Object | complete JSON configuration object for getUserMedia(<userMediaConfiguration>) eg: { "audio": true, "video": { "width": { "max": "<width>" }, "height": { "max": "<height>" }, "frameRate": { "min": "<fps>"}}}

### 9. 'eventLeave'
***

### Description
***
Event received when a user quits the roomId.

### Syntax
***

#### *Event*
```json
{
  "a"  :  "eventLeave",
  "d"  :  {
            "roomSize"   :  <roomSize>,
            "socketId"   :  "<socketId>",
            "userId"     :  "<userId>"
          }
}
```
Name | Type | Description
---- | ---- | ----
roomId | String | Room identifier
roomSize | Integer | Number of peers connected to the room (updated)
socketId | String | Socket Identifier that quit the roomId
userId | String | backend userId associated to this socketId

### 10. 'eventOrientationChange'
***

### Description
***
event sent when a peer has changed is orientation and send orientationChange API message. this event will be received by all other peers connected to the same roomId. Then some actions could be taken on views if something is wrong.

### Syntax
***

#### *Event*
```json
{
  "a"  :  "eventOrientationChange",
  "d"  :  {
            "from"          :  {
               "socketId" : "<socketId>",
               "userId"   : "<userId>"
            },
            "platform"      :  "<platform>",
            "orientation"   :  <orientation>,
            "camera"        :  "<camera>"
          }
}
```
Name | Type | Description
---- | ---- | ----
from | JSON Object | From which peer event came
socketId | String | Socket Identifier (32 bytes in hexadecimal string)
userId | String | backend userId (short id) associated to the socketId
platform | String | Platform used to join ("Android","iOS","Web")
orientation | Integer | New orientation of the device (0, 90, 180, 270)
camera | String | Orientation of the camera ("front" or "back")

### 11. 'eventFreeze'
***

### Description
***
event sent when a video freeze occured (eg fps to 0 / bad connection)

### Syntax
***

#### *Event*
```json
{
  "a"  :  "eventFreeze"
}
```
Name | Type | Description
---- | ---- | ----
No data fields

### 12. 'eventCpu'
***

### Description
***
event sent by the client app every 60 seconds with the instant CPU used (%)

### Syntax
***

#### *Event*
```json
{
  "a"  :  "eventCpu",
  "d"  :  {
            "cpuUsed": <cpu>        
  }
}
```
Name | Type | Description
---- | ---- | ----
cpuUsed | Integer | CPU consumption in %

## RabbitMQ

This micro service send events on an exchange named live_events. If you would like to receive these events, you should create a queue bound to this exchange name. Routing keys represent the event name.

### Events name (Routing Keys)

- room.join
- room.leave
- room.live

### room.join

### Description
***
A user has joined the roomId

### Syntax
***

```json
{
  "roomId"     :  "<roomId>",
  "roomSize"   :  <roomSize>,
  "socketId"   :  "<socketId>",
  "userId"     :  "<userId>",
  "platform"   :  "<platform>",
  "appVersion" : "<appVersion>",
  "version"    :  "<version>",
  "ip"         :  "<ip>",
  "bitrate"    :  <bitrate>
}
```
Name | Type | Description
---- | ---- | ----
roomId | String | Room identifier
roomSize | Integer | Number of peers connected to the room (updated)
socketId | String | Socket Identifier that joined the roomId
userId | String | backend userId associated to this socketId
platform | String | type of platform (eg: "Web", "iOS", "Android")
appVersion | String | version of the client app
version | String | version of the browser or OS mobile device

### room.leave

### Description
***
A user has left the roomId

### Syntax
***

```json
{
  "roomId"   :  "<roomId>",
  "roomSize" :  <roomSize>,
  "socketId" :  "<socketId>",
   "userId"  :  "<userId>"
}
```
Name | Type | Description
---- | ---- | ----
roomId | String | Room identifier
roomSize | Integer | Number of peers connected to the room (updated)
socketId | String | Socket Identifier that quit the roomId
userId | String | backend userId associated to this socketId

### room.live

### Description
***
This event is sent every 60 seconds and rebroadcasts all Sessions running on a specified roomId.

### Syntax
***

```json
{
  "roomId"   :  "<roomId>",
  "roomSize" :  <roomSize>,
  "sessions" :  [
    {
      "socketId": "<socketId>",
      "userId"  : "<userId>"
    },
    ...
  ]
}
```
Name | Type | Description
---- | ---- | ----
roomId | String | Room identifier
roomSize | Integer | Number of peers connected to the room (updated)
sessions | Object Array | an array of sessions attached to the roomId
socketId | String | Socket Identifier (32 bytes in hexadecimal string)
userId | String | backend userId (short id) associated to the socketId
