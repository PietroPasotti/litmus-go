package lib

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/litmuschaos/litmus-go/pkg/clients"
	jujuCommon "github.com/litmuschaos/litmus-go/pkg/cloud/juju/common"
	jujuOps "github.com/litmuschaos/litmus-go/pkg/cloud/juju/operations"
	"github.com/litmuschaos/litmus-go/pkg/events"
	experimentTypes "github.com/litmuschaos/litmus-go/pkg/juju/add-unit/types"
	"github.com/litmuschaos/litmus-go/pkg/log"
	"github.com/litmuschaos/litmus-go/pkg/probe"
	"github.com/litmuschaos/litmus-go/pkg/types"
	"github.com/litmuschaos/litmus-go/pkg/utils/common"
	"github.com/palantir/stacktrace"
)

var (
	inject, abort chan os.Signal
)

// PrepareJujuAddUnit contains the preparation and injection steps for the experiment
func PrepareJujuAddUnit(ctx context.Context, experimentsDetails *experimentTypes.ExperimentDetails, clients clients.ClientSets, resultDetails *types.ResultDetails, eventsDetails *types.EventDetails, chaosDetails *types.ChaosDetails) error {

	inject = make(chan os.Signal, 1)
	signal.Notify(inject, os.Interrupt, syscall.SIGTERM)
	abort = make(chan os.Signal, 1)
	signal.Notify(abort, os.Interrupt, syscall.SIGTERM)

	if experimentsDetails.RampTime != 0 {
		log.Infof("[Ramp]: Waiting for the %vs ramp time before injecting chaos", experimentsDetails.RampTime)
		common.WaitForDuration(experimentsDetails.RampTime)
	}

	creds, err := jujuCommon.GetJujuCredentials(experimentsDetails.CredentialsFile)
	if err != nil {
		return stacktrace.Propagate(err, "failed to get Juju credentials")
	}

	client, err := jujuCommon.NewJujuClient(creds, experimentsDetails.ModelUUID)
	if err != nil {
		return stacktrace.Propagate(err, "failed to connect to Juju controller")
	}
	defer client.Close()

	go abortWatcher(experimentsDetails, chaosDetails)

	select {
	case <-inject:
		os.Exit(0)
	default:
		if experimentsDetails.EngineName != "" {
			msg := "Injecting " + experimentsDetails.ExperimentName + " chaos on Juju model"
			types.SetEngineEventAttributes(eventsDetails, types.ChaosInject, msg, "Normal", chaosDetails)
			events.GenerateEvents(eventsDetails, clients, chaosDetails, "ChaosEngine")
		}

		log.Infof("[Chaos]: Adding %d unit(s) to application %s", experimentsDetails.NumUnits, experimentsDetails.AppName)
		if err := jujuOps.AddUnit(ctx, client, experimentsDetails.AppName, experimentsDetails.NumUnits); err != nil {
			return stacktrace.Propagate(err, "failed to add units")
		}

		common.SetTargets(experimentsDetails.AppName, "injected", "JujuUnit", chaosDetails)

		if len(resultDetails.ProbeDetails) != 0 {
			if err := probe.RunProbes(ctx, chaosDetails, clients, resultDetails, "DuringChaos", eventsDetails); err != nil {
				return stacktrace.Propagate(err, "failed to run probes")
			}
		}

		common.SetTargets(experimentsDetails.AppName, "reverted", "JujuUnit", chaosDetails)
	}

	if experimentsDetails.RampTime != 0 {
		log.Infof("[Ramp]: Waiting for the %vs ramp time after injecting chaos", experimentsDetails.RampTime)
		common.WaitForDuration(experimentsDetails.RampTime)
	}
	return nil
}

func abortWatcher(experimentsDetails *experimentTypes.ExperimentDetails, chaosDetails *types.ChaosDetails) {
	<-abort
	log.Info("[Abort]: Chaos Revert Started")
	log.Info("[Abort]: No automatic revert for add-unit; manual intervention may be needed")
	common.SetTargets(experimentsDetails.AppName, "reverted", "JujuUnit", chaosDetails)
	log.Info("[Abort]: Chaos Revert Completed")
	os.Exit(1)
}
