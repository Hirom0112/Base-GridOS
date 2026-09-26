package main

import (
	"log"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/dispatch"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	temporalClient, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer temporalClient.Close()

	dispatchWorker := worker.New(temporalClient, dispatch.TaskQueue, worker.Options{})
	dispatchWorker.RegisterWorkflow(dispatch.Workflow)
	if err = dispatchWorker.Run(worker.InterruptCh()); err != nil {
		log.Fatal(err)
	}
}
