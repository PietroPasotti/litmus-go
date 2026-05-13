package lib

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	jujuCommon "github.com/litmuschaos/litmus-go/pkg/cloud/juju/common"
	jujuOps "github.com/litmuschaos/litmus-go/pkg/cloud/juju/operations"
	experimentTypes "github.com/litmuschaos/litmus-go/pkg/juju/set-config/types"
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

// PrepareJujuSetConfig contains the preparation and injection steps for the experiment
func PrepareJujuSetConfig(ctx context.Context, experimentsDetails *experimentTypes.ExperimentDetails, clients clients.ClientSets, resultDetails *types.ResultDetails, eventsDetails *types.EventDetails, chaosDetails *types.ChaosDetails) error {

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

	config := parseConfig(experimentsDetails.ConfigKeys)

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

		log.Infof("[Chaos]: Setting config on application %s: %v", experimentsDetails.AppName, config)
		if err := jujuOps.SetConfig(ctx, client, experimentsDetails.AppName, config); err != nil {
			return stacktrace.Propagate(err, "failed to set config")
		}

		common.SetTargets(experimentsDetails.AppName, "injected", "JujuConfig", chaosDetails)

		if len(resultDetails.ProbeDetails) != 0 {
			if err := probe.RunProbes(ctx, chaosDetails, clients, resultDetails, "DuringChaos", eventsDetails); err != nil {
				return stacktrace.Propagate(err, "failed to run probes")
			}
		}

		common.SetTargets(experimentsDetails.AppName, "reverted", "JujuConfig", chaosDetails)
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

func abortWatcher(experimentsDetails *experimentTypes.ExperimentDetails, chaosDetails *types.ChaosDetails) {
	<-abort
	log.Info("[Abort]: Chaos Revert Started")
	log.Info("[Abort]: No automatic revert for set-config; manual intervention may be needed")
	common.SetTargets(experimentsDetails.AppName, "reverted", "JujuConfig", chaosDetails)
	log.Info("[Abort]: Chaos Revert Completed")
	os.Exit(1)
}
