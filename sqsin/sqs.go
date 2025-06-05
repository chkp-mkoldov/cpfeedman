package sqsin

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSIn represents SQS input - consumes messages and executes callbac function with the message body

// allows to define a callback function to handle messages -
// swqIn.OnMessage = func(msg *types.Message) {
// 	fmt.Fprintf(os.Stdout, "CALLBACK Received message: %s\n", *msg.Body
// ...

// expecting SQS queue URL
// AWS is authenticated via environment variables

type SQSIn struct {
	QueueUrl  string                   // SQS queue URL
	OnMessage func(msg *types.Message) // Callback function to handle received messages
}

func NewSQSIn(queueUrl string) *SQSIn {
	return &SQSIn{
		QueueUrl:  queueUrl,
		OnMessage: nil, // default to nil, can be set later
	}
}

func (s *SQSIn) Listen() error {
	// fmt.Println("Starting SQS Client...")

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("[SQS] unable to load SDK config, %v", err)
	}

	client := sqs.NewFromConfig(cfg)

	fmt.Printf("[SQSIN] Client initialized with Queue URL: %s\n", s.QueueUrl)

	ctx := context.TODO()

	for {
		output, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(s.QueueUrl),
			MaxNumberOfMessages: 1,
			AttributeNames:      []types.QueueAttributeName{"SentTimestamp"},
		})
		if err != nil {
			log.Printf("[SQSIN] error receiving message: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if len(output.Messages) == 0 {
			continue // no message
		}

		for _, msg := range output.Messages {
			log.Printf("[SQSIN] Received message: %s", aws.ToString(msg.Body))

			// delegete to callback function if set
			if s.OnMessage != nil {
				s.OnMessage(&msg)
			}

			// Delete message
			_, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(s.QueueUrl),
				ReceiptHandle: msg.ReceiptHandle,
			})
			if err != nil {
				log.Printf("[SQSIN] failed to delete message: %v", err)
			} else {
				log.Printf("[SQSIN] Deleted message ID: %s", aws.ToString(msg.MessageId))
			}
		}
	}

}
