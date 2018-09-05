#!/bin/sh

#
# usage
# ./run.sh
# ./run.sh builder
# ./run.sh bash
# ./run.sh builder bash
#

if [ $# -eq 0 ]
then
  # ./run.sh
  # @see infra-dockercompose docker-compose.yml file
  docker run -ti --network=host \
    --env INSTANCE_UUID=localhost \
    --env FULL_UNIT_NAME=live-webrtcsignaling@1.service \
    --env CERT_FILE_PATH=/go/src/app/star_tribedev.pm.crt \
    --env KEY_FILE_PATH=/go/src/app/star_tribedev.pm.key \
    --env UNIT_NUMBER=1 \
    --env JWT_SECRET=IvteOzwBqBRICDgOt61pRSXCLHkj0wYkSaIMbwvbts6ah3Rpqc \
    --env GRAPHITE_IPV4=graphite.tribedev.pm \
    --env RABBITMQ_URL=amqp://backend:backend2016@localhost:15672 \
    --env PORT_NUMBER=1443 \
    --env PUBLIC_IPV4=127.0.0.1 \
    --env GST_DEBUG=3 \
    --env GST_DEBUG_DUMP_DOT_DIR=/go/src/app/ \
    live-webrtcsignaling
elif [ $# -eq 1 ] && [ $1 = "builder" ]
then
  # ./run.sh builder
  docker run -ti --network=host \
    --env INSTANCE_UUID=localhost \
    --env FULL_UNIT_NAME=live-webrtcsignaling@1.service \
    --env CERT_FILE_PATH=/go/src/app/star_tribedev.pm.crt \
    --env KEY_FILE_PATH=/go/src/app/star_tribedev.pm.key \
    --env UNIT_NUMBER=1 \
    --env JWT_SECRET=IvteOzwBqBRICDgOt61pRSXCLHkj0wYkSaIMbwvbts6ah3Rpqc \
    --env GRAPHITE_IPV4=graphite.tribedev.pm \
    --env RABBITMQ_URL=amqp://backend:backend2016@localhost:15672 \
    --env PORT_NUMBER=1443 \
    --env PUBLIC_IPV4=127.0.0.1 \
    --env GST_DEBUG=3 \
    --env GST_DEBUG_DUMP_DOT_DIR=/go/src/app/ \
    live-webrtcsignaling:builder
elif [ $# -eq 1 ] && [  $1 = "bash" ]
then
  # ./run.sh bash
  docker run -ti live-webrtcsignaling bash
elif [ $# -eq 2 ] && [  $1 = "builder" ] && [  $2 = "bash" ]
then
  # ./run.sh builder bash
  docker run -ti live-webrtcsignaling:builder bash
else
  echo "unknown command"
fi
