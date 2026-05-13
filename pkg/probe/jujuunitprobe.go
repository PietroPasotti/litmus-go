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

func prepareJujuUnitProbe(probe v1alpha1.ProbeAttributes, clients clients.ClientSets, chaosDetails *types.ChaosDetails, resultDetails *types.ResultDetails, phase string) error {
	switch strings.ToLower(phase) {
	case "prechaos":
		return preChaosJujuUnitProbe(probe, resultDetails, clients, chaosDetails)
	case "postchaos":
		return postChaosJujuUnitProbe(probe, resultDetails, clients, chaosDetails)
	case "duringchaos":
		return onChaosJujuUnitProbe(probe, resultDetails, clients, chaosDetails)
	default:
		return cerrors.Error{ErrorCode: cerrors.ErrorTypeJujuUnitProbe, Target: fmt.Sprintf("{name: %v}", probe.Name), Reason: fmt.Sprintf("phase '%s' not supported in the juju unit probe", phase)}
	}
}

func triggerJujuUnitProbe(probe v1alpha1.ProbeAttributes, resultDetails *types.ResultDetails) error {
	probeTimeout := getProbeTimeouts(probe.Name, resultDetails.ProbeDetails)
	inputs := probe.JujuUnitProbeInputs

	var description string
	if err := retry.Times(uint(getAttempts(probe.RunProperties.Attempt, probe.RunProperties.Retry))).
		Timeout(probeTimeout.ProbeTimeout).
		Wait(probeTimeout.Interval).
		TryWithTimeout(func(attempt uint) error {
			credsFile := getJujuCredentialsFile()
			creds, err := common.GetJujuCredentials(credsFile)
			if err != nil {
				return cerrors.Error{ErrorCode: cerrors.ErrorTypeJujuUnitProbe, Target: fmt.Sprintf("{name: %v}", probe.Name), Reason: fmt.Sprintf("failed to get juju credentials: %v", err)}
			}
			client, err := common.NewJujuClient(creds, inputs.ModelUUID)
			if err != nil {
				return cerrors.Error{ErrorCode: cerrors.ErrorTypeJujuUnitProbe, Target: fmt.Sprintf("{name: %v}", probe.Name), Reason: fmt.Sprintf("failed to create juju client: %v", err)}
			}
			defer client.Close()

			result, err := jujuOps.GetUnitStatus(context.Background(), client, inputs.UnitName)
			if err != nil {
				return cerrors.Error{ErrorCode: cerrors.ErrorTypeJujuUnitProbe, Target: fmt.Sprintf("{name: %v}", probe.Name), Reason: fmt.Sprintf("failed to get unit status: %v", err)}
			}

			rc := getAndIncrementRunCount(resultDetails, probe.Name)
			description, err = validateJujuProbeResult(inputs.Comparator, probe.Name, probe.RunProperties.Verbosity, result.Current, rc, cerrors.FailureTypeJujuUnitProbe)
			if err != nil {
				return err
			}

			probes := types.ProbeArtifact{}
			probes.ProbeArtifacts.Register = fmt.Sprintf("status=%s,message=%s,since=%s", result.Current, result.Message, result.Since)
			resultDetails.ProbeArtifacts[probe.Name] = probes
			return nil
		}); err != nil {
		return checkProbeTimeoutError(probe.Name, cerrors.FailureTypeJujuUnitProbe, err)
	}

	setProbeDescription(resultDetails, probe, description)
	return nil
}

func preChaosJujuUnitProbe(probe v1alpha1.ProbeAttributes, resultDetails *types.ResultDetails, clients clients.ClientSets, chaosDetails *types.ChaosDetails) error {
	probeTimeout := getProbeTimeouts(probe.Name, resultDetails.ProbeDetails)

	switch strings.ToLower(probe.Mode) {
	case "sot", "edge":
		log.InfoWithValues("[Probe]: Juju unit probe information", logrus.Fields{
			"Name":         probe.Name,
			"UnitName":     probe.JujuUnitProbeInputs.UnitName,
			"TargetStatus": probe.JujuUnitProbeInputs.TargetStatus,
			"Mode":         probe.Mode,
			"Phase":        "PreChaos",
		})
		if probeTimeout.InitialDelay != 0 {
			log.Infof("[Wait]: Waiting for %v before probe execution", probe.RunProperties.InitialDelay)
			time.Sleep(probeTimeout.InitialDelay)
		}
		if err = triggerJujuUnitProbe(probe, resultDetails); err != nil && cerrors.GetErrorType(err) != cerrors.FailureTypeJujuUnitProbe {
			return err
		}
		if err = markedVerdictInEnd(err, resultDetails, probe, "PreChaos"); err != nil {
			return err
		}
	case "continuous":
		log.InfoWithValues("[Probe]: Juju unit probe information", logrus.Fields{
			"Name":         probe.Name,
			"UnitName":     probe.JujuUnitProbeInputs.UnitName,
			"TargetStatus": probe.JujuUnitProbeInputs.TargetStatus,
			"Mode":         probe.Mode,
			"Phase":        "PreChaos",
		})
		go triggerContinuousJujuUnitProbe(probe, clients, resultDetails, chaosDetails)
	}
	return nil
}

func postChaosJujuUnitProbe(probe v1alpha1.ProbeAttributes, resultDetails *types.ResultDetails, clients clients.ClientSets, chaosDetails *types.ChaosDetails) error {
	probeTimeout := getProbeTimeouts(probe.Name, resultDetails.ProbeDetails)

	switch strings.ToLower(probe.Mode) {
	case "eot", "edge":
		log.InfoWithValues("[Probe]: Juju unit probe information", logrus.Fields{
			"Name":         probe.Name,
			"UnitName":     probe.JujuUnitProbeInputs.UnitName,
			"TargetStatus": probe.JujuUnitProbeInputs.TargetStatus,
			"Mode":         probe.Mode,
			"Phase":        "PostChaos",
		})
		if probeTimeout.InitialDelay != 0 {
			log.Infof("[Wait]: Waiting for %v before probe execution", probe.RunProperties.InitialDelay)
			time.Sleep(probeTimeout.InitialDelay)
		}
		if err = triggerJujuUnitProbe(probe, resultDetails); err != nil && cerrors.GetErrorType(err) != cerrors.FailureTypeJujuUnitProbe {
			return err
		}
		if err = markedVerdictInEnd(err, resultDetails, probe, "PostChaos"); err != nil {
			return err
		}
	case "continuous", "onchaos":
		if err = checkForErrorInContinuousProbe(resultDetails, probe.Name, chaosDetails.Delay, chaosDetails.Timeout); err != nil && cerrors.GetErrorType(err) != cerrors.FailureTypeJujuUnitProbe && cerrors.GetErrorType(err) != cerrors.FailureTypeProbeTimeout {
			return err
		}
		if err = markedVerdictInEnd(err, resultDetails, probe, "PostChaos"); err != nil {
			return err
		}
	}
	return nil
}

func onChaosJujuUnitProbe(probe v1alpha1.ProbeAttributes, resultDetails *types.ResultDetails, clients clients.ClientSets, chaosDetails *types.ChaosDetails) error {
	switch strings.ToLower(probe.Mode) {
	case "onchaos":
		log.InfoWithValues("[Probe]: Juju unit probe information", logrus.Fields{
			"Name":         probe.Name,
			"UnitName":     probe.JujuUnitProbeInputs.UnitName,
			"TargetStatus": probe.JujuUnitProbeInputs.TargetStatus,
			"Mode":         probe.Mode,
			"Phase":        "DuringChaos",
		})
		go triggerOnChaosJujuUnitProbe(probe, clients, resultDetails, chaosDetails)
	}
	return nil
}

func triggerContinuousJujuUnitProbe(probe v1alpha1.ProbeAttributes, clients clients.ClientSets, chaosresult *types.ResultDetails, chaosDetails *types.ChaosDetails) {
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
			if errCheck := triggerJujuUnitProbe(probe, chaosresult); errCheck != nil {
				isExperimentFailed = true
				break loop
			}
			time.Sleep(probeTimeout.ProbePollingInterval)
		}
	}
	if p := getProbeByName(probe.Name, chaosresult.ProbeDetails); p != nil {
		p.HasProbeCompleted = true
		if isExperimentFailed {
			p.IsProbeFailedWithError = fmt.Errorf("juju unit probe %s failed during continuous mode", probe.Name)
		}
	}
}

func triggerOnChaosJujuUnitProbe(probe v1alpha1.ProbeAttributes, clients clients.ClientSets, chaosresult *types.ResultDetails, chaosDetails *types.ChaosDetails) {
	probeTimeout := getProbeTimeouts(probe.Name, chaosresult.ProbeDetails)
	ctx := chaosDetails.ProbeContext.Ctx
	var isExperimentFailed bool
	loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		default:
			if errCheck := triggerJujuUnitProbe(probe, chaosresult); errCheck != nil {
				isExperimentFailed = true
				break loop
			}
			time.Sleep(probeTimeout.ProbePollingInterval)
		}
	}
	if p := getProbeByName(probe.Name, chaosresult.ProbeDetails); p != nil {
		p.HasProbeCompleted = true
		if isExperimentFailed {
			p.IsProbeFailedWithError = fmt.Errorf("juju unit probe %s failed during onchaos mode", probe.Name)
		}
	}
}
