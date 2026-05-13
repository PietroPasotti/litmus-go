package probe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	"github.com/litmuschaos/litmus-go/pkg/cerrors"
	"github.com/litmuschaos/litmus-go/pkg/clients"
	"github.com/litmuschaos/litmus-go/pkg/cloud/juju/common"
	jujuOps "github.com/litmuschaos/litmus-go/pkg/cloud/juju/operations"
	"github.com/litmuschaos/litmus-go/pkg/log"
	"github.com/litmuschaos/litmus-go/pkg/types"
	"github.com/litmuschaos/litmus-go/pkg/utils/retry"
	"github.com/sirupsen/logrus"
)

func prepareJujuPebbleProbe(probe v1alpha1.ProbeAttributes, clients clients.ClientSets, chaosDetails *types.ChaosDetails, resultDetails *types.ResultDetails, phase string) error {
	switch strings.ToLower(phase) {
	case "prechaos":
		return preChaosJujuPebbleProbe(probe, resultDetails, clients, chaosDetails)
	case "postchaos":
		return postChaosJujuPebbleProbe(probe, resultDetails, clients, chaosDetails)
	case "duringchaos":
		return onChaosJujuPebbleProbe(probe, resultDetails, clients, chaosDetails)
	default:
		return cerrors.Error{ErrorCode: cerrors.ErrorTypeJujuPebbleProbe, Target: fmt.Sprintf("{name: %v}", probe.Name), Reason: fmt.Sprintf("phase '%s' not supported in the juju pebble probe", phase)}
	}
}

func triggerJujuPebbleProbe(probe v1alpha1.ProbeAttributes, resultDetails *types.ResultDetails) error {
	probeTimeout := getProbeTimeouts(probe.Name, resultDetails.ProbeDetails)
	inputs := probe.JujuPebbleProbeInputs

	var description string
	if err := retry.Times(uint(getAttempts(probe.RunProperties.Attempt, probe.RunProperties.Retry))).
		Timeout(probeTimeout.ProbeTimeout).
		Wait(probeTimeout.Interval).
		TryWithTimeout(func(attempt uint) error {
			credsFile := getJujuCredentialsFile()
			creds, err := common.GetJujuCredentials(credsFile)
			if err != nil {
				return cerrors.Error{ErrorCode: cerrors.ErrorTypeJujuPebbleProbe, Target: fmt.Sprintf("{name: %v}", probe.Name), Reason: fmt.Sprintf("failed to get juju credentials: %v", err)}
			}
			client, err := common.NewJujuClient(creds, inputs.ModelUUID)
			if err != nil {
				return cerrors.Error{ErrorCode: cerrors.ErrorTypeJujuPebbleProbe, Target: fmt.Sprintf("{name: %v}", probe.Name), Reason: fmt.Sprintf("failed to create juju client: %v", err)}
			}
			defer client.Close()

			result, err := jujuOps.GetPebbleServiceStatus(context.Background(), client, inputs.UnitName, inputs.ServiceName)
			if err != nil {
				return cerrors.Error{ErrorCode: cerrors.ErrorTypeJujuPebbleProbe, Target: fmt.Sprintf("{name: %v}", probe.Name), Reason: fmt.Sprintf("failed to get pebble service status: %v", err)}
			}

			rc := getAndIncrementRunCount(resultDetails, probe.Name)
			description, err = validateJujuProbeResult(inputs.Comparator, probe.Name, probe.RunProperties.Verbosity, result.Current, rc, cerrors.FailureTypeJujuPebbleProbe)
			if err != nil {
				return err
			}

			probes := types.ProbeArtifact{}
			probes.ProbeArtifacts.Register = fmt.Sprintf("service=%s,status=%s", inputs.ServiceName, result.Current)
			resultDetails.ProbeArtifacts[probe.Name] = probes
			return nil
		}); err != nil {
		return checkProbeTimeoutError(probe.Name, cerrors.FailureTypeJujuPebbleProbe, err)
	}

	setProbeDescription(resultDetails, probe, description)
	return nil
}

func preChaosJujuPebbleProbe(probe v1alpha1.ProbeAttributes, resultDetails *types.ResultDetails, clients clients.ClientSets, chaosDetails *types.ChaosDetails) error {
	probeTimeout := getProbeTimeouts(probe.Name, resultDetails.ProbeDetails)

	switch strings.ToLower(probe.Mode) {
	case "sot", "edge":
		log.InfoWithValues("[Probe]: Juju pebble probe information", logrus.Fields{
			"Name":        probe.Name,
			"UnitName":    probe.JujuPebbleProbeInputs.UnitName,
			"ServiceName": probe.JujuPebbleProbeInputs.ServiceName,
			"TargetState": probe.JujuPebbleProbeInputs.TargetState,
			"Mode":        probe.Mode,
			"Phase":       "PreChaos",
		})
		if probeTimeout.InitialDelay != 0 {
			log.Infof("[Wait]: Waiting for %v before probe execution", probe.RunProperties.InitialDelay)
			time.Sleep(probeTimeout.InitialDelay)
		}
		if err = triggerJujuPebbleProbe(probe, resultDetails); err != nil && cerrors.GetErrorType(err) != cerrors.FailureTypeJujuPebbleProbe {
			return err
		}
		if err = markedVerdictInEnd(err, resultDetails, probe, "PreChaos"); err != nil {
			return err
		}
	case "continuous":
		log.InfoWithValues("[Probe]: Juju pebble probe information", logrus.Fields{
			"Name":        probe.Name,
			"UnitName":    probe.JujuPebbleProbeInputs.UnitName,
			"ServiceName": probe.JujuPebbleProbeInputs.ServiceName,
			"TargetState": probe.JujuPebbleProbeInputs.TargetState,
			"Mode":        probe.Mode,
			"Phase":       "PreChaos",
		})
		go triggerContinuousJujuPebbleProbe(probe, clients, resultDetails, chaosDetails)
	}
	return nil
}

func postChaosJujuPebbleProbe(probe v1alpha1.ProbeAttributes, resultDetails *types.ResultDetails, clients clients.ClientSets, chaosDetails *types.ChaosDetails) error {
	probeTimeout := getProbeTimeouts(probe.Name, resultDetails.ProbeDetails)

	switch strings.ToLower(probe.Mode) {
	case "eot", "edge":
		log.InfoWithValues("[Probe]: Juju pebble probe information", logrus.Fields{
			"Name":        probe.Name,
			"UnitName":    probe.JujuPebbleProbeInputs.UnitName,
			"ServiceName": probe.JujuPebbleProbeInputs.ServiceName,
			"TargetState": probe.JujuPebbleProbeInputs.TargetState,
			"Mode":        probe.Mode,
			"Phase":       "PostChaos",
		})
		if probeTimeout.InitialDelay != 0 {
			log.Infof("[Wait]: Waiting for %v before probe execution", probe.RunProperties.InitialDelay)
			time.Sleep(probeTimeout.InitialDelay)
		}
		if err = triggerJujuPebbleProbe(probe, resultDetails); err != nil && cerrors.GetErrorType(err) != cerrors.FailureTypeJujuPebbleProbe {
			return err
		}
		if err = markedVerdictInEnd(err, resultDetails, probe, "PostChaos"); err != nil {
			return err
		}
	case "continuous", "onchaos":
		if err = checkForErrorInContinuousProbe(resultDetails, probe.Name, chaosDetails.Delay, chaosDetails.Timeout); err != nil && cerrors.GetErrorType(err) != cerrors.FailureTypeJujuPebbleProbe && cerrors.GetErrorType(err) != cerrors.FailureTypeProbeTimeout {
			return err
		}
		if err = markedVerdictInEnd(err, resultDetails, probe, "PostChaos"); err != nil {
			return err
		}
	}
	return nil
}

func onChaosJujuPebbleProbe(probe v1alpha1.ProbeAttributes, resultDetails *types.ResultDetails, clients clients.ClientSets, chaosDetails *types.ChaosDetails) error {
	switch strings.ToLower(probe.Mode) {
	case "onchaos":
		log.InfoWithValues("[Probe]: Juju pebble probe information", logrus.Fields{
			"Name":        probe.Name,
			"UnitName":    probe.JujuPebbleProbeInputs.UnitName,
			"ServiceName": probe.JujuPebbleProbeInputs.ServiceName,
			"TargetState": probe.JujuPebbleProbeInputs.TargetState,
			"Mode":        probe.Mode,
			"Phase":       "DuringChaos",
		})
		go triggerOnChaosJujuPebbleProbe(probe, clients, resultDetails, chaosDetails)
	}
	return nil
}

func triggerContinuousJujuPebbleProbe(probe v1alpha1.ProbeAttributes, clients clients.ClientSets, chaosresult *types.ResultDetails, chaosDetails *types.ChaosDetails) {
	probeTimeout := getProbeTimeouts(probe.Name, chaosresult.ProbeDetails)
	ctx, cancel := context.WithCancel(context.Background())
	chaosDetails.ProbeContext.Ctx = ctx
	chaosDetails.ProbeContext.CancelFunc = cancel

	var isExperimentFailed bool
	loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		default:
			if errCheck := triggerJujuPebbleProbe(probe, chaosresult); errCheck != nil {
				isExperimentFailed = true
				break loop
			}
			time.Sleep(probeTimeout.ProbePollingInterval)
		}
	}
	if p := getProbeByName(probe.Name, chaosresult.ProbeDetails); p != nil {
		p.HasProbeCompleted = true
		if isExperimentFailed {
			p.IsProbeFailedWithError = fmt.Errorf("juju pebble probe %s failed during continuous mode", probe.Name)
		}
	}
}

func triggerOnChaosJujuPebbleProbe(probe v1alpha1.ProbeAttributes, clients clients.ClientSets, chaosresult *types.ResultDetails, chaosDetails *types.ChaosDetails) {
	probeTimeout := getProbeTimeouts(probe.Name, chaosresult.ProbeDetails)
	ctx := chaosDetails.ProbeContext.Ctx
	var isExperimentFailed bool
	loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		default:
			if errCheck := triggerJujuPebbleProbe(probe, chaosresult); errCheck != nil {
				isExperimentFailed = true
				break loop
			}
			time.Sleep(probeTimeout.ProbePollingInterval)
		}
	}
	if p := getProbeByName(probe.Name, chaosresult.ProbeDetails); p != nil {
		p.HasProbeCompleted = true
		if isExperimentFailed {
			p.IsProbeFailedWithError = fmt.Errorf("juju pebble probe %s failed during onchaos mode", probe.Name)
		}
	}
}
