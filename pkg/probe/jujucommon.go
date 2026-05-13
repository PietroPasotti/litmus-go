package probe

import (
	"fmt"
	"strings"

	"github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	"github.com/litmuschaos/litmus-go/pkg/cerrors"
	cmp "github.com/litmuschaos/litmus-go/pkg/probe/comparator"
	"github.com/litmuschaos/litmus-go/pkg/types"
)

const defaultJujuCredentialsFile = "/etc/juju-creds/juju-credentials.yaml"

func getJujuCredentialsFile() string {
	creds := types.Getenv("JUJU_CREDENTIALS_FILE", "")
	if creds != "" {
		return creds
	}
	return defaultJujuCredentialsFile
}

func validateJujuProbeResult(comparator v1alpha1.ComparatorInfo, probeName, probeVerbosity string, actualValue string, rc int, failureType cerrors.ErrorType) (string, error) {
	if comparator.Value == "" {
		return fmt.Sprintf("Actual value: '%s'", actualValue), nil
	}

	compare := cmp.RunCount(rc).
		FirstValue(actualValue).
		SecondValue(comparator.Value).
		Criteria(comparator.Criteria).
		ProbeName(probeName).
		ProbeVerbosity(probeVerbosity)

	switch strings.ToLower(comparator.Type) {
	case "int":
		if err := compare.CompareInt(failureType); err != nil {
			return "", err
		}
	case "float":
		if err := compare.CompareFloat(failureType); err != nil {
			return "", err
		}
	case "string", "":
		if err := compare.CompareString(failureType); err != nil {
			return "", err
		}
	default:
		return "", cerrors.Error{ErrorCode: failureType, Target: fmt.Sprintf("{name: %v}", probeName), Reason: fmt.Sprintf("comparator type '%s' not supported", comparator.Type)}
	}
	return fmt.Sprintf("Actual value: '%s'. Expected value: '%s'", actualValue, comparator.Value), nil
}

func isJujuExperiment(experimentName string) bool {
	return strings.HasPrefix(experimentName, "juju-")
}
