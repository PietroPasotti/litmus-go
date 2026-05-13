package types

import (
	clientTypes "k8s.io/apimachinery/pkg/types"
)

// ExperimentDetails contains the experiment-related details for juju-add-app
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
	CharmName       string
	AppName         string
	Channel         string
	NumUnits        int
	AppConfig       string
	ModelUUID        string
	CredentialsFile string
	Sequence        string
}
