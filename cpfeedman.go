package main

import (
	"cpfeedman/config"
	"cpfeedman/cpapi"
	"cpfeedman/sqsin"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

const version = "v0.1.0"

// global config variable
var cfg config.Config

// CP API client
var cpApi *cpapi.CpApi

// init configuration and more
func init() {
	// Load configuration from environment variables
	cfg.LoadFromEnv()
	cpApi = cpapi.NewCpApiFromConfig(&cfg)

}

// check active feeds on each gateway
func mapFeedsOnGateways(gwNames []string) {

	fmt.Fprintln(os.Stdout, "[FeedMap] Mapping active feeds on each gateway. This may take a while, please wait...")

	// execute mapping active feeds on each gateway
	resp, err := cpApi.RunScript("(date; hostname; dynamic_objects -efo_show | grep -Po '^object name : \\K.*') | tee -a /var/log/cpfeedman.log", "map feeds", gwNames)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error running script on Check Point API:", err)
		os.Exit(1)
	}
	// fmt.Fprintln(os.Stdout, "RunScript response:", resp.GetTaskIds())

	unfinishedTasks := resp.GetTaskIds()

	loopStartTime := time.Now()
	for len(unfinishedTasks) > 0 {
		// fmt.Fprintln(os.Stdout, "Waiting for tasks to finish:", unfinishedTasks)
		time.Sleep(1 * time.Second)

		taskRes, err := cpApi.ShowTasks(resp.GetTaskIds())
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error getting task results from Check Point API:", err)
			os.Exit(1)
		}
		// fmt.Fprintln(os.Stdout, "GetTaskResults response:", taskRes)
		// fmt.Fprintln(os.Stdout, "Tasks by status:", taskRes.GetTasksByStatus())

		// response message for finished tasks
		finishedTasks := taskRes.GetFinishedTasksDetail()
		// fmt.Fprintln(os.Stdout, "Finished tasks:", finishedTasks)
		for _, taskDetail := range finishedTasks {
			// fmt.Println("FINISHED Task ID:", taskDetail.TaskID)
			responseMessage := taskDetail.GetTaskResponseMessage()
			// fmt.Println("Response message:", responseMessage)
			if responseMessage != "" {
				fmt.Fprintf(os.Stdout, "\n[FeedMap] Task %s finished with message:\n===\n%s===\n", taskDetail.TaskID, responseMessage)
			}
		}

		unfinishedTasks = taskRes.GetUnfinishedTaskIds()
		if len(unfinishedTasks) == 0 {
			fmt.Fprintln(os.Stdout, "[FeedMap] All tasks finished successfully.")
			break
		}

		if time.Since(loopStartTime) > 2*time.Minute { //
			fmt.Fprintln(os.Stderr, "[FeedMap] Timeout waiting for tasks to finish. Exiting.")
			break
		}
	}

}

func main() {
	fmt.Fprintln(os.Stdout, "cpfeedman version", version)

	fmt.Fprintln(os.Stdout, "Check Point Management Server:", cfg.CheckPointServer)

	// retrieve all gateways and feeds
	gwNames, err := cpApi.GatewayNames()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error fetching from Check Point API:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "gwNames:", gwNames)

	feedNames, err := cpApi.FeedNames()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error fetching from Check Point API:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "feedNames:", feedNames)

	fmt.Fprintln(os.Stdout, "")
	mapFeedsOnGateways(gwNames)

	// logoutResponse, err := cpApi.Logout()
	// if err != nil {
	// 	fmt.Fprintln(os.Stderr, "Error logging out from Check Point API:", err)
	// 	os.Exit(1)
	// }
	// fmt.Fprintln(os.Stdout, "Logout response:", logoutResponse)

	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "[SQS] Listening for SQS messages")

	sqsIn := sqsin.NewSQSIn(cfg.CpFeedManSqsEndpoint)

	sqsIn.OnMessage = func(msg *types.Message) {
		fmt.Fprintf(os.Stdout, "\n")
		fmt.Fprintf(os.Stdout, "[SQS] CALLBACK Received message: %s\n", *msg.Body)

		// Here you can process the message, e.g., parse it, run scripts, etc.

		// is msg.Body in feedNames?
		if msg.Body != nil {
			for _, feedName := range feedNames {
				if *msg.Body == feedName {
					fmt.Fprintf(os.Stdout, "[SQS] Message body '%s' matches feed name '%s'.\n", *msg.Body, feedName)

					// TODO rate limiting
					// TODO feed map - ask only relevant gateways (vs all)
					res, err := cpApi.KickFeed(feedName, gwNames)
					if err != nil {
						fmt.Fprintf(os.Stderr, "[SQS] Error kicking feed '%s': %v\n", feedName, err)
						continue
					} else {
						fmt.Fprintf(os.Stdout, "[SQS] Kick feed response for '%s': %s\n", feedName, res)
					}

					fmt.Fprintf(os.Stdout, "[SQS] Kicked feed '%s'.\n", feedName)

				}
			}
		} else {
			fmt.Fprintln(os.Stderr, "[SQS] Received message with nil body.")
		}

		fmt.Fprintf(os.Stdout, "\n")
	}

	if err := sqsIn.Listen(); err != nil {
		fmt.Fprintln(os.Stderr, "[SQS] Error listening on SQS:", err)
		os.Exit(1)
	}
}
