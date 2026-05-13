package types

import (
	clientTypes "k8s.io/apimachinery/pkg/types"
)

// ExperimentDetails contains the experiment-related details for juju-set-config
type ExperimentDetails struct {
	ExperimentName  string
	EngineName      string
	RampTime        int
	ChaosDuration   int
	ChaosInterval   int
	ChaosUID        clientTypes.UID
	ChaosNamespace  string
	ChaosPodName    string
	Timeout         int
	Delay           int
	AppName         string
	ConfigKeys      string
	ModelUUID        string
	CredentialsFile string
	Sequence        string
}
