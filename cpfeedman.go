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

// global config variable
var cfg config.Config

// init configuration and more
func init() {
	// Load configuration from environment variables
	cfg.LoadFromEnv()
}

func main() {
	fmt.Println("Hello, World!")

	fmt.Fprintln(os.Stdout, "Check Point Server:", cfg.CheckPointServer)

	cpApi := cpapi.NewCpApiFromConfig(&cfg)

	// loginResponse, err := cpApi.Login()
	// if err != nil {
	// 	fmt.Fprintln(os.Stderr, "Error logging in to Check Point API:", err)
	// 	os.Exit(1)
	// }
	// fmt.Fprintln(os.Stdout, "Login response:", loginResponse)

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

	resp, err := cpApi.RunScript("(date; hostname; dynamic_objects -efo_show | grep -Po '^object name : \\K.*') | tee -a /var/log/cpfeedman.log", "log date", gwNames)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error running script on Check Point API:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "RunScript response:", resp.GetTaskIds())

	unfinishedTasks := resp.GetTaskIds()

	loopStartTime := time.Now()
	for len(unfinishedTasks) > 0 {
		fmt.Fprintln(os.Stdout, "Waiting for tasks to finish:", unfinishedTasks)
		time.Sleep(1 * time.Second)

		taskRes, err := cpApi.ShowTasks(resp.GetTaskIds())
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error getting task results from Check Point API:", err)
			os.Exit(1)
		}
		// fmt.Fprintln(os.Stdout, "GetTaskResults response:", taskRes)
		fmt.Fprintln(os.Stdout, "Tasks by status:", taskRes.GetTasksByStatus())

		// response message for finished tasks
		finishedTasks := taskRes.GetFinishedTasksDetail()
		// fmt.Fprintln(os.Stdout, "Finished tasks:", finishedTasks)
		for _, taskDetail := range finishedTasks {
			fmt.Println("FINISHED Task ID:", taskDetail.TaskID)
			responseMessage := taskDetail.GetTaskResponseMessage()
			// fmt.Println("Response message:", responseMessage)
			if responseMessage != "" {
				fmt.Fprintf(os.Stdout, "Task %s finished with message:\n===\n%s===\n", taskDetail.TaskID, responseMessage)
			}
		}

		unfinishedTasks = taskRes.GetUnfinishedTaskIds()
		if len(unfinishedTasks) == 0 {
			fmt.Fprintln(os.Stdout, "All tasks finished successfully.")
			break
		}

		if time.Since(loopStartTime) > 2*time.Minute { //
			fmt.Fprintln(os.Stderr, "Timeout waiting for tasks to finish. Exiting.")
			break
		}
	}

	logoutResponse, err := cpApi.Logout()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error logging out from Check Point API:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "Logout response:", logoutResponse)

	fmt.Fprintln(os.Stdout, "Listening on SQS.")

	swsIn := sqsin.NewSQSIn(cfg.CpFeedManSqsEndpoint)

	swsIn.OnMessage = func(msg *types.Message) {
		fmt.Fprintf(os.Stdout, "CALLBACK Received message: %s\n", *msg.Body)

		// Here you can process the message, e.g., parse it, run scripts, etc.

		// is msg.Body in feedNames?
		if msg.Body != nil {
			for _, feedName := range feedNames {
				if *msg.Body == feedName {
					fmt.Fprintf(os.Stdout, "Message body '%s' matches feed name '%s'.\n", *msg.Body, feedName)

					// TODO rate limiting
					// TODO feed map - ask only relevant gateways (vs all)
					res, err := cpApi.KickFeed(feedName, gwNames)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error kicking feed '%s': %v\n", feedName, err)
						continue
					} else {
						fmt.Fprintf(os.Stdout, "KickFeed response for '%s': %s\n", feedName, res)
					}

					fmt.Fprintf(os.Stdout, "Kicked feed '%s'.\n", feedName)

				}
			}
		} else {
			fmt.Fprintln(os.Stderr, "Received message with nil body.")
		}
	}

	if err := swsIn.Listen(); err != nil {
		fmt.Fprintln(os.Stderr, "Error listening on SQS:", err)
		os.Exit(1)
	}
}
