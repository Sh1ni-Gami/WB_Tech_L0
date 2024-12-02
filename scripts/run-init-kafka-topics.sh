#!/bin/sh

KT="/opt/bitnami/kafka/bin/kafka-topics.sh" 

sh -c "echo \"Waiting for kafka...\" && \"$KT\" --bootstrap-server localhost:9092 --list && echo \"Creating kafka topics\" && \"$KT\" --bootstrap-server localhost:9092 --create --if-not-exists --topic $KAFKA_TOPIC --replication-factor 1 --partitions 1 && echo \"Successfully created the following topics:\" && \"$KT\" --bootstrap-server localhost:9092 --list" & 

exit 0
