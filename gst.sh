#!/bin/sh

GST_DEBUG=4 gst-launch-1.0 -v rtpbin name=rtpbin rtp-profile=4 \
udpsrc port=10001 caps="application/x-rtp,media=(string)video,payload=(int)96,clock-rate=(int)90000,encoding-name=(string)VP8" ! rtpbin.recv_rtp_sink_0 \
rtpbin. ! rtpvp8depay ! vp8dec ! x264enc ! queue ! mpegtsmux name=tsmux ! queue ! filesink location=prout.ts \
udpsrc port=10002 ! rtpbin.recv_rtcp_sink_0 \
udpsrc port=10003 caps="application/x-rtp,media=(string)audio,payload=(int)111,clock-rate=(int)48000,encoding-name=(string)OPUS" ! rtpbin.recv_rtp_sink_1 \
rtpbin. ! rtpopusdepay ! opusdec ! voaacenc ! queue ! tsmux. \
udpsrc port=10004 ! rtpbin.recv_rtcp_sink_1

#rtpbin. ! rtpvp8depay ! vp8dec ! x264enc ! mp4mux faststart=1 name=mpmux ! fakesink dump=1
#rtpbin. ! rtpopusdepay ! opusdec ! voaacenc ! queue ! mpmux. \
