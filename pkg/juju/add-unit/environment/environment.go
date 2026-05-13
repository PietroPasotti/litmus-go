package environment

import (
	"strconv"

	clientTypes "k8s.io/apimachinery/pkg/types"

	experimentTypes "github.com/litmuschaos/litmus-go/pkg/juju/add-unit/types"
	"github.com/litmuschaos/litmus-go/pkg/types"
)

// GetENV fetches all the env variables from the runner pod
func GetENV(experimentDetails *experimentTypes.ExperimentDetails) {
	experimentDetails.ExperimentName = types.Getenv("EXPERIMENT_NAME", "juju-add-unit")
	experimentDetails.ChaosNamespace = types.Getenv("CHAOS_NAMESPACE", "litmus")
	experimentDetails.EngineName = types.Getenv("CHAOSENGINE", "")
	experimentDetails.ChaosDuration, _ = strconv.Atoi(types.Getenv("TOTAL_CHAOS_DURATION", "30"))
	experimentDetails.ChaosInterval, _ = strconv.Atoi(types.Getenv("CHAOS_INTERVAL", "30"))
	experimentDetails.RampTime, _ = strconv.Atoi(types.Getenv("RAMP_TIME", "0"))
	experimentDetails.ChaosUID = clientTypes.UID(types.Getenv("CHAOS_UID", ""))
	experimentDetails.ChaosPodName = types.Getenv("POD_NAME", "")
	experimentDetails.Delay, _ = strconv.Atoi(types.Getenv("STATUS_CHECK_DELAY", "2"))
	experimentDetails.Timeout, _ = strconv.Atoi(types.Getenv("STATUS_CHECK_TIMEOUT", "180"))
	experimentDetails.AppName = types.Getenv("APP_NAME", "")
	experimentDetails.NumUnits, _ = strconv.Atoi(types.Getenv("NUM_UNITS", "1"))
	experimentDetails.ModelUUID = types.Getenv("JUJU_MODEL_UUID", "")
	experimentDetails.CredentialsFile = types.Getenv("JUJU_CREDENTIALS_FILE", "/tmp/juju-credentials.yaml")
	experimentDetails.Sequence = types.Getenv("SEQUENCE", "parallel")
}
