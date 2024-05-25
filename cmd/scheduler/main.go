package main

import (
	"flag"
	"log"

	"github.com/sagoresarker/task-queue/pkg/common"
	"github.com/sagoresarker/task-queue/pkg/schedular"
)

var (
	schedulerPort = flag.String("scheduler_port", ":8081", "Port on which the Scheduler serves requests.")
)

func main() {
	dbConnectionString := common.GetDBConnectionString()
	schedulerServer := schedular.NewServer(*schedulerPort, dbConnectionString)
	err := schedulerServer.Start()
	if err != nil {
		log.Fatalf("Error while starting server: %+v", err)
	}
}
