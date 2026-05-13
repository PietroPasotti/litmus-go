package types

// JujuCommonDetails contains fields shared across all Juju chaos experiments
type JujuCommonDetails struct {
	ControllerAddress string
	CACert            string
	Username          string
	Password          string
	ModelUUID         string
	CredentialsFile   string
}
