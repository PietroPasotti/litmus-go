package lib

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	jujuCommon "github.com/litmuschaos/litmus-go/pkg/cloud/juju/common"
	jujuOps "github.com/litmuschaos/litmus-go/pkg/cloud/juju/operations"
	experimentTypes "github.com/litmuschaos/litmus-go/pkg/juju/remove-relation/types"
	
	"github.com/litmuschaos/litmus-go/pkg/clients"
	"github.com/litmuschaos/litmus-go/pkg/events"
	"github.com/litmuschaos/litmus-go/pkg/log"
	"github.com/litmuschaos/litmus-go/pkg/probe"
	"github.com/litmuschaos/litmus-go/pkg/types"
	"github.com/litmuschaos/litmus-go/pkg/utils/common"
	"github.com/palantir/stacktrace"
)

var (
	inject, abort chan os.Signal
)

// PrepareJujuRemoveRelation contains the preparation and injection steps for the experiment
func PrepareJujuRemoveRelation(ctx context.Context, experimentsDetails *experimentTypes.ExperimentDetails, clients clients.ClientSets, resultDetails *types.ResultDetails, eventsDetails *types.EventDetails, chaosDetails *types.ChaosDetails) error {

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

	go abortWatcher(ctx, experimentsDetails, client, chaosDetails)

	select {
	case <-inject:
		os.Exit(0)
	default:
		if experimentsDetails.EngineName != "" {
			msg := "Injecting " + experimentsDetails.ExperimentName + " chaos on Juju model"
			types.SetEngineEventAttributes(eventsDetails, types.ChaosInject, msg, "Normal", chaosDetails)
			events.GenerateEvents(eventsDetails, clients, chaosDetails, "ChaosEngine")
		}

		log.Infof("[Chaos]: Removing relation between %s and %s", experimentsDetails.Endpoint1, experimentsDetails.Endpoint2)
		if err := jujuOps.RemoveRelation(ctx, client, experimentsDetails.Endpoint1, experimentsDetails.Endpoint2); err != nil {
			return stacktrace.Propagate(err, "failed to remove relation")
		}

		common.SetTargets(experimentsDetails.Endpoint1+":"+experimentsDetails.Endpoint2, "injected", "JujuRelation", chaosDetails)

		if len(resultDetails.ProbeDetails) != 0 {
			if err := probe.RunProbes(ctx, chaosDetails, clients, resultDetails, "DuringChaos", eventsDetails); err != nil {
				return stacktrace.Propagate(err, "failed to run probes")
			}
		}

		log.Infof("[Wait]: Waiting for %vs chaos duration", experimentsDetails.ChaosDuration)
		common.WaitForDuration(experimentsDetails.ChaosDuration)

		log.Infof("[Revert]: Re-integrating %s with %s", experimentsDetails.Endpoint1, experimentsDetails.Endpoint2)
		if err := jujuOps.Integrate(ctx, client, experimentsDetails.Endpoint1, experimentsDetails.Endpoint2); err != nil {
			log.Errorf("Failed to revert relation removal: %v", err)
		}

		common.SetTargets(experimentsDetails.Endpoint1+":"+experimentsDetails.Endpoint2, "reverted", "JujuRelation", chaosDetails)
	}

	if experimentsDetails.RampTime != 0 {
		log.Infof("[Ramp]: Waiting for the %vs ramp time after injecting chaos", experimentsDetails.RampTime)
		common.WaitForDuration(experimentsDetails.RampTime)
	}
	return nil
}

func abortWatcher(ctx context.Context, experimentsDetails *experimentTypes.ExperimentDetails, client *jujuCommon.JujuClient, chaosDetails *types.ChaosDetails) {
	<-abort
	log.Info("[Abort]: Chaos Revert Started")
	log.Infof("[Abort]: Re-integrating %s with %s", experimentsDetails.Endpoint1, experimentsDetails.Endpoint2)
	if err := jujuOps.Integrate(ctx, client, experimentsDetails.Endpoint1, experimentsDetails.Endpoint2); err != nil {
		log.Errorf("Failed to revert relation removal on abort: %v", err)
	}
	common.SetTargets(experimentsDetails.Endpoint1+":"+experimentsDetails.Endpoint2, "reverted", "JujuRelation", chaosDetails)
	log.Info("[Abort]: Chaos Revert Completed")
	os.Exit(1)
}
