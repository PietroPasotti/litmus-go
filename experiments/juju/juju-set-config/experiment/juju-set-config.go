package experiment

import (
	"context"
	"os"

	litmusLIB "github.com/litmuschaos/litmus-go/chaoslib/litmus/juju-set-config/lib"
	experimentEnv "github.com/litmuschaos/litmus-go/pkg/juju/set-config/environment"
	experimentTypes "github.com/litmuschaos/litmus-go/pkg/juju/set-config/types"
	"github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	"github.com/litmuschaos/litmus-go/pkg/clients"
	"github.com/litmuschaos/litmus-go/pkg/events"
	"github.com/litmuschaos/litmus-go/pkg/log"
	"github.com/litmuschaos/litmus-go/pkg/probe"
	"github.com/litmuschaos/litmus-go/pkg/result"
	"github.com/litmuschaos/litmus-go/pkg/types"
	"github.com/litmuschaos/litmus-go/pkg/utils/common"
	"github.com/sirupsen/logrus"
)

// JujuSetConfig is the entry point for the juju-set-config chaos experiment
func JujuSetConfig(ctx context.Context, clients clients.ClientSets) {

	experimentsDetails := experimentTypes.ExperimentDetails{}
	resultDetails := types.ResultDetails{}
	eventsDetails := types.EventDetails{}
	chaosDetails := types.ChaosDetails{}

	log.Infof("[PreReq]: Getting the ENV for the %v experiment", os.Getenv("EXPERIMENT_NAME"))
	experimentEnv.GetENV(&experimentsDetails)

	types.InitialiseChaosVariables(&chaosDetails)
	types.SetResultAttributes(&resultDetails, chaosDetails)

	if experimentsDetails.EngineName != "" {
		if err := common.GetValuesFromChaosEngine(&chaosDetails, clients, &resultDetails); err != nil {
			log.Errorf("Unable to initialize the probes: %v", err)
			return
		}
	}

	log.Infof("[PreReq]: Updating the chaos result of %v experiment (SOT)", experimentsDetails.ExperimentName)
	if err := result.ChaosResult(&chaosDetails, clients, &resultDetails, "SOT"); err != nil {
		log.Errorf("Unable to create the chaosresult: %v", err)
		result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
		return
	}

	result.SetResultUID(&resultDetails, clients, &chaosDetails)

	msg := "experiment: " + experimentsDetails.ExperimentName + ", Result: Awaited"
	types.SetResultEventAttributes(&eventsDetails, types.AwaitedVerdict, msg, "Normal", &resultDetails)
	if eventErr := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosResult"); eventErr != nil {
		log.Errorf("Failed to create %v event inside chaosresult", types.AwaitedVerdict)
	}

	log.InfoWithValues("The experiment information is as follows", logrus.Fields{
		"Chaos Duration":  experimentsDetails.ChaosDuration,
		"Chaos Namespace": experimentsDetails.ChaosNamespace,
		"App Name":        experimentsDetails.AppName,
		"Config Keys":     experimentsDetails.ConfigKeys,
	})

	go common.AbortWatcherWithoutExit(experimentsDetails.ExperimentName, clients, &resultDetails, &chaosDetails, &eventsDetails)

	if experimentsDetails.EngineName != "" {
		msg := "AUT: Running"
		if len(resultDetails.ProbeDetails) != 0 {
			if err := probe.RunProbes(ctx, &chaosDetails, clients, &resultDetails, "PreChaos", &eventsDetails); err != nil {
				log.Errorf("Probe Failed: %v", err)
				msg := "AUT: Running, Probes: Unsuccessful"
				types.SetEngineEventAttributes(&eventsDetails, types.PreChaosCheck, msg, "Warning", &chaosDetails)
				if eventErr := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine"); eventErr != nil {
					log.Errorf("Failed to create %v event inside chaosengine", types.PreChaosCheck)
				}
				result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
				return
			}
			msg = "AUT: Running, Probes: Successful"
		}
		types.SetEngineEventAttributes(&eventsDetails, types.PreChaosCheck, msg, "Normal", &chaosDetails)
		if eventErr := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine"); eventErr != nil {
			log.Errorf("Failed to create %v event inside chaosengine", types.PreChaosCheck)
		}
	}

	chaosDetails.Phase = types.ChaosInjectPhase

	if err := litmusLIB.PrepareJujuSetConfig(ctx, &experimentsDetails, clients, &resultDetails, &eventsDetails, &chaosDetails); err != nil {
		log.Errorf("Chaos injection failed: %v", err)
		result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
		return
	}

	log.Infof("[Confirmation]: %v chaos has been injected successfully", experimentsDetails.ExperimentName)
	resultDetails.Verdict = v1alpha1.ResultVerdictPassed
	chaosDetails.Phase = types.PostChaosPhase

	if experimentsDetails.EngineName != "" {
		msg := "AUT: Running"
		if len(resultDetails.ProbeDetails) != 0 {
			if err := probe.RunProbes(ctx, &chaosDetails, clients, &resultDetails, "PostChaos", &eventsDetails); err != nil {
				log.Errorf("Probes Failed: %v", err)
				msg := "AUT: Running, Probes: Unsuccessful"
				types.SetEngineEventAttributes(&eventsDetails, types.PostChaosCheck, msg, "Warning", &chaosDetails)
				if eventErr := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine"); eventErr != nil {
					log.Errorf("Failed to create %v event inside chaosengine", types.PostChaosCheck)
				}
				result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
				return
			}
			msg = "AUT: Running, Probes: Successful"
		}
		types.SetEngineEventAttributes(&eventsDetails, types.PostChaosCheck, msg, "Normal", &chaosDetails)
		if eventErr := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine"); eventErr != nil {
			log.Errorf("Failed to create %v event inside chaosengine", types.PostChaosCheck)
		}
	}

	log.Infof("[The End]: Updating the chaos result of %v experiment (EOT)", experimentsDetails.ExperimentName)
	if err := result.ChaosResult(&chaosDetails, clients, &resultDetails, "EOT"); err != nil {
		log.Errorf("Unable to update the chaosresult: %v", err)
		return
	}

	msg = "experiment: " + experimentsDetails.ExperimentName + ", Result: " + string(resultDetails.Verdict)
	reason, eventType := types.GetChaosResultVerdictEvent(resultDetails.Verdict)
	types.SetResultEventAttributes(&eventsDetails, reason, msg, eventType, &resultDetails)
	if eventErr := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosResult"); eventErr != nil {
		log.Errorf("Failed to create %v event inside chaosresult", reason)
	}

	if experimentsDetails.EngineName != "" {
		msg := experimentsDetails.ExperimentName + " experiment has been " + string(resultDetails.Verdict) + "ed"
		types.SetEngineEventAttributes(&eventsDetails, types.Summary, msg, "Normal", &chaosDetails)
		if eventErr := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine"); eventErr != nil {
			log.Errorf("Failed to create %v event inside chaosengine", types.Summary)
		}
	}
}
