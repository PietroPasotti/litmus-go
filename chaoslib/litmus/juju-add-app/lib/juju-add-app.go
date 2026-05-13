package lib

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	jujuCommon "github.com/litmuschaos/litmus-go/pkg/cloud/juju/common"
	jujuOps "github.com/litmuschaos/litmus-go/pkg/cloud/juju/operations"
	experimentTypes "github.com/litmuschaos/litmus-go/pkg/juju/add-app/types"
	
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

// PrepareJujuAddApp contains the preparation and injection steps for the experiment
func PrepareJujuAddApp(ctx context.Context, experimentsDetails *experimentTypes.ExperimentDetails, clients clients.ClientSets, resultDetails *types.ResultDetails, eventsDetails *types.EventDetails, chaosDetails *types.ChaosDetails) error {

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

	// Parse config from "key1=val1,key2=val2" format
	config := parseConfig(experimentsDetails.AppConfig)

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

		log.Infof("[Chaos]: Deploying charm %s as %s", experimentsDetails.CharmName, experimentsDetails.AppName)
		if err := jujuOps.AddApp(ctx, client, experimentsDetails.CharmName, experimentsDetails.AppName, experimentsDetails.Channel, experimentsDetails.NumUnits, config); err != nil {
			return stacktrace.Propagate(err, "failed to deploy application")
		}

		common.SetTargets(experimentsDetails.AppName, "injected", "JujuApp", chaosDetails)

		if len(resultDetails.ProbeDetails) != 0 {
			if err := probe.RunProbes(ctx, chaosDetails, clients, resultDetails, "DuringChaos", eventsDetails); err != nil {
				return stacktrace.Propagate(err, "failed to run probes")
			}
		}

		common.SetTargets(experimentsDetails.AppName, "reverted", "JujuApp", chaosDetails)
	}

	if experimentsDetails.RampTime != 0 {
		log.Infof("[Ramp]: Waiting for the %vs ramp time after injecting chaos", experimentsDetails.RampTime)
		common.WaitForDuration(experimentsDetails.RampTime)
	}
	return nil
}

// parseConfig parses a "key1=val1,key2=val2" string into a map
func parseConfig(configStr string) map[string]string {
	config := make(map[string]string)
	if configStr == "" {
		return config
	}
	pairs := strings.Split(configStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			config[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return config
}

func abortWatcher(ctx context.Context, experimentsDetails *experimentTypes.ExperimentDetails, client *jujuCommon.JujuClient, chaosDetails *types.ChaosDetails) {
	<-abort
	log.Info("[Abort]: Chaos Revert Started")
	log.Infof("[Abort]: Removing application %s", experimentsDetails.AppName)
	if err := jujuOps.RemoveApp(ctx, client, experimentsDetails.AppName); err != nil {
		log.Errorf("Failed to revert app deployment on abort: %v", err)
	}
	common.SetTargets(experimentsDetails.AppName, "reverted", "JujuApp", chaosDetails)
	log.Info("[Abort]: Chaos Revert Completed")
	os.Exit(1)
}
