#!/bin/bash

### Sending sample messages to SQS queue during testing

SQS_REGION="eu-north-1"
SQS_NAME="MyTestQueue"

SQS_QUEUE_URL=$(aws sqs get-queue-url --queue-name "${SQS_NAME}" --region "${SQS_REGION}" --output text)
echo "SQS queue URL: $SQS_QUEUE_URL"

function send_message() {
    local message=$1
    echo "Sending message: $message"
    aws sqs send-message --queue-url "$SQS_QUEUE_URL" \
      --message-body "$message" \
      --region "$SQS_REGION" | jq -c .
}

while true; do
    send_message "quiccloud"
    sleep 2

    send_message "feedME"
    sleep 2

    send_message "invalid message on $(date)"
    sleep 10; 
done