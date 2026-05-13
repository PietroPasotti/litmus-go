package common

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// JujuCredentials holds the credentials needed to connect to a Juju controller.
type JujuCredentials struct {
	ControllerAddr string `yaml:"controller-addr"`
	ControllerUUID string `yaml:"controller-uuid"`
	CACert         string `yaml:"ca-cert"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	ModelUUID      string `yaml:"model-uuid"`
}

// GetJujuCredentials reads Juju credentials from a YAML file.
func GetJujuCredentials(credentialsFile string) (*JujuCredentials, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file %s: %w", credentialsFile, err)
	}

	var creds JujuCredentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file %s: %w", credentialsFile, err)
	}

	if creds.ControllerAddr == "" {
		return nil, fmt.Errorf("controller-addr is required in credentials file")
	}
	if creds.Username == "" {
		return nil, fmt.Errorf("username is required in credentials file")
	}
	if creds.Password == "" {
		return nil, fmt.Errorf("password is required in credentials file")
	}

	return &creds, nil
}

// JujuClient wraps the juju CLI binary for executing commands against a Juju controller.
type JujuClient struct {
	Credentials *JujuCredentials
	ModelUUID   string
	ModelName   string // resolved name for -m flag (e.g. "admin/sut-test")
	DataDir     string // JUJU_DATA directory with config files
}

// NewJujuClient creates a new JujuClient configured with the given credentials and model.
// It writes the necessary juju config files (controllers.yaml, accounts.yaml, etc.)
// so that the juju CLI can authenticate without interactive login.
func NewJujuClient(creds *JujuCredentials, modelUUID string) (*JujuClient, error) {
	if creds == nil {
		return nil, fmt.Errorf("credentials cannot be nil")
	}
	if modelUUID == "" {
		modelUUID = creds.ModelUUID
	}

	// Create a JUJU_DATA directory. Avoid /tmp as it may be a read-only secret mount.
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "/litmus"
	}
	dataDir := filepath.Join(homeDir, ".local", "share", "juju")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create juju data dir: %w", err)
	}

	controllerName := "chaos-controller"

	// Write controllers.yaml
	controllersYAML := fmt.Sprintf(`controllers:
  %s:
    uuid: "%s"
    api-endpoints: ["%s"]
    ca-cert: |
%s
current-controller: %s
`, controllerName, creds.ControllerUUID, creds.ControllerAddr, indentCert(creds.CACert), controllerName)

	if err := os.WriteFile(filepath.Join(dataDir, "controllers.yaml"), []byte(controllersYAML), 0600); err != nil {
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("failed to write controllers.yaml: %w", err)
	}

	// Write accounts.yaml
	accountsYAML := fmt.Sprintf(`controllers:
  %s:
    user: %s
    password: %s
`, controllerName, creds.Username, creds.Password)

	if err := os.WriteFile(filepath.Join(dataDir, "accounts.yaml"), []byte(accountsYAML), 0600); err != nil {
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("failed to write accounts.yaml: %w", err)
	}

	// Write models.yaml if we have a model UUID
	if modelUUID != "" {
		modelsYAML := fmt.Sprintf(`controllers:
  %s:
    models:
      default:
        uuid: %s
    current-model: default
`, controllerName, modelUUID)

		if err := os.WriteFile(filepath.Join(dataDir, "models.yaml"), []byte(modelsYAML), 0600); err != nil {
			os.RemoveAll(dataDir)
			return nil, fmt.Errorf("failed to write models.yaml: %w", err)
		}
	}

	client := &JujuClient{
		Credentials: creds,
		ModelUUID:   modelUUID,
		DataDir:     dataDir,
	}

	// Resolve model UUID to model name. The juju CLI -m flag requires
	// a model name (e.g. "admin/sut-test"), not a UUID.
	if modelUUID != "" {
		modelName, err := client.resolveModelName(modelUUID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: could not resolve model name for UUID %s: %v\n", modelUUID, err)
		} else {
			client.ModelName = modelName
		}
	}

	return client, nil
}

// indentCert indents each line of the CA cert by 6 spaces for YAML embedding.
func indentCert(cert string) string {
	if cert == "" {
		return "      \"\""
	}
	lines := strings.Split(strings.TrimSpace(cert), "\n")
	for i, line := range lines {
		lines[i] = "      " + line
	}
	return strings.Join(lines, "\n")
}

type jujuModelsOutput struct {
	Models []struct {
		ShortName string `json:"short-name"`
		ModelUUID string `json:"model-uuid"`
		Name      string `json:"name"`
	} `json:"models"`
}

func (c *JujuClient) resolveModelName(uuid string) (string, error) {
	cmd := exec.Command("juju", "models", "--format=json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("JUJU_DATA=%s", c.DataDir))
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("juju models failed: %w\nStderr: %s", err, stderr.String())
	}
	var parsed jujuModelsOutput
	if err := json.Unmarshal([]byte(stdout.String()), &parsed); err != nil {
		return "", fmt.Errorf("failed to parse juju models output: %w", err)
	}
	for _, m := range parsed.Models {
		if m.ModelUUID == uuid {
			if m.Name != "" {
				return m.Name, nil
			}
			return m.ShortName, nil
		}
	}
	return "", fmt.Errorf("model with UUID %s not found", uuid)
}

func (c *JujuClient) Run(ctx context.Context, args ...string) (string, error) {
	modelRef := c.ModelName
	if modelRef == "" {
		modelRef = c.ModelUUID
	}
	if modelRef != "" && len(args) > 0 {
		subCmd := args[0]
		modelCommands := map[string]bool{
			"remove-relation":    true,
			"integrate":          true,
			"add-unit":           true,
			"remove-unit":        true,
			"deploy":             true,
			"remove-application": true,
			"config":             true,
			"run":                true,
			"status":             true,
		}
		if modelCommands[subCmd] {
			rest := args[1:]
			args = append([]string{subCmd, "-m", modelRef}, rest...)
		}
	}

	cmd := exec.CommandContext(ctx, "juju", args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("JUJU_DATA=%s", c.DataDir),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("juju %s failed: %w\nOutput: %s", strings.Join(args, " "), err, string(output))
	}

	return string(output), nil
}

// RunJSON executes a juju CLI command with --format=json and unmarshals the result.
func (c *JujuClient) RunJSON(ctx context.Context, result interface{}, args ...string) error {
	args = append(args, "--format=json")
	output, err := c.Run(ctx, args...)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(output), result)
}

// Close cleans up the temporary JUJU_DATA directory.
func (c *JujuClient) Close() error {
	if c.DataDir != "" {
		return os.RemoveAll(c.DataDir)
	}
	return nil
}
