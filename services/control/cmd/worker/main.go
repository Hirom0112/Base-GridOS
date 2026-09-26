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
	dispatchWorker.RegisterActivity(dispatch.FreezeInputs)
	dispatchWorker.RegisterActivity(dispatch.RequestPlan)
	dispatchWorker.RegisterActivity(dispatch.ValidatePlan)
	dispatchWorker.RegisterActivity(dispatch.PersistIntents)
	dispatchWorker.RegisterActivity(dispatch.PublishCommands)
	dispatchWorker.RegisterActivity(dispatch.TrackAcknowledgements)
	dispatchWorker.RegisterActivity(dispatch.VerifyDelivery)
	dispatchWorker.RegisterActivity(dispatch.EndEvent)
	dispatchWorker.RegisterActivity(dispatch.ReconcileLateMessages)
	dispatchWorker.RegisterActivity(dispatch.ProduceReport)
	dispatchWorker.RegisterActivity(dispatch.IssueEmergencyStop)
	if err = dispatchWorker.Run(worker.InterruptCh()); err != nil {
		log.Fatal(err)
	}
}
