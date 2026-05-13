package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/litmuschaos/litmus-go/pkg/cloud/juju/common"
	"github.com/litmuschaos/litmus-go/pkg/log"
)

// Integrate creates a relation between two application endpoints.
// Equivalent to: juju integrate <endpoint1> <endpoint2>
func Integrate(ctx context.Context, client *common.JujuClient, endpoint1, endpoint2 string) error {
	log.Infof("[Juju] Integrating %s with %s", endpoint1, endpoint2)
	_, err := client.Run(ctx, "integrate", endpoint1, endpoint2)
	if err != nil {
		return fmt.Errorf("failed to integrate %s with %s: %w", endpoint1, endpoint2, err)
	}
	log.Infof("[Juju] Successfully integrated %s with %s", endpoint1, endpoint2)
	return nil
}

// RemoveRelation removes a relation between two application endpoints.
// Equivalent to: juju remove-relation <endpoint1> <endpoint2>
func RemoveRelation(ctx context.Context, client *common.JujuClient, endpoint1, endpoint2 string) error {
	log.Infof("[Juju] Removing relation between %s and %s", endpoint1, endpoint2)
	_, err := client.Run(ctx, "remove-relation", endpoint1, endpoint2)
	if err != nil {
		return fmt.Errorf("failed to remove relation between %s and %s: %w", endpoint1, endpoint2, err)
	}
	log.Infof("[Juju] Successfully removed relation between %s and %s", endpoint1, endpoint2)
	return nil
}

// AddUnit adds units to a Juju application.
// Equivalent to: juju add-unit <app> -n <numUnits>
func AddUnit(ctx context.Context, client *common.JujuClient, app string, numUnits int) error {
	log.Infof("[Juju] Adding %d unit(s) to application %s", numUnits, app)
	_, err := client.Run(ctx, "add-unit", app, "-n", fmt.Sprintf("%d", numUnits))
	if err != nil {
		return fmt.Errorf("failed to add units to %s: %w", app, err)
	}
	log.Infof("[Juju] Successfully added %d unit(s) to %s", numUnits, app)
	return nil
}

// RemoveUnit removes specific units from a Juju application.
// Equivalent to: juju remove-unit <unit1> <unit2> ...
func RemoveUnit(ctx context.Context, client *common.JujuClient, unitIDs []string) error {
	log.Infof("[Juju] Removing units: %v", unitIDs)
	args := append([]string{"remove-unit"}, unitIDs...)
	_, err := client.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to remove units %v: %w", unitIDs, err)
	}
	log.Infof("[Juju] Successfully removed units: %v", unitIDs)
	return nil
}

// AddApp deploys a new application from a charm.
// Equivalent to: juju deploy <charm> <appName> --channel <channel> -n <numUnits>
func AddApp(ctx context.Context, client *common.JujuClient, charm, appName, channel string, numUnits int, config map[string]string) error {
	log.Infof("[Juju] Deploying charm %s as %s (channel: %s, units: %d)", charm, appName, channel, numUnits)

	args := []string{"deploy", charm, appName, "-n", fmt.Sprintf("%d", numUnits)}
	if channel != "" {
		args = append(args, "--channel", channel)
	}
	for k, v := range config {
		args = append(args, "--config", fmt.Sprintf("%s=%s", k, v))
	}

	_, err := client.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to deploy %s: %w", charm, err)
	}
	log.Infof("[Juju] Successfully deployed %s as %s", charm, appName)
	return nil
}

// RemoveApp removes (destroys) a Juju application.
// Equivalent to: juju remove-application <app>
func RemoveApp(ctx context.Context, client *common.JujuClient, app string) error {
	log.Infof("[Juju] Removing application %s", app)
	_, err := client.Run(ctx, "remove-application", app)
	if err != nil {
		return fmt.Errorf("failed to remove application %s: %w", app, err)
	}
	log.Infof("[Juju] Successfully removed application %s", app)
	return nil
}

// SetConfig sets configuration values on a Juju application.
// Equivalent to: juju config <app> key=value ...
func SetConfig(ctx context.Context, client *common.JujuClient, app string, config map[string]string) error {
	log.Infof("[Juju] Setting config on application %s: %v", app, config)
	args := []string{"config", app}
	for k, v := range config {
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}
	_, err := client.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to set config on %s: %w", app, err)
	}
	log.Infof("[Juju] Successfully set config on application %s", app)
	return nil
}

// RunAction executes an action on a Juju unit and waits for it to complete.
// Equivalent to: juju run <unit> <actionName> [params]
func RunAction(ctx context.Context, client *common.JujuClient, unit, actionName string, params map[string]string) error {
	log.Infof("[Juju] Running action %s on unit %s", actionName, unit)
	args := []string{"run", unit, actionName}
	for k, v := range params {
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}
	output, err := client.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to run action %s on %s: %w", actionName, unit, err)
	}
	log.Infof("[Juju] Action output: %s", output)
	return nil
}

// JujuStatus represents the relevant fields from `juju status --format=json`.
type JujuStatus struct {
	Applications map[string]ApplicationStatus `json:"applications"`
}

// ApplicationStatus represents an application's status in `juju status`.
type ApplicationStatus struct {
	ApplicationStatus StatusInfo            `json:"application-status"`
	Units             map[string]UnitStatus `json:"units,omitempty"`
}

// UnitStatus represents a unit's status in `juju status`.
type UnitStatus struct {
	WorkloadStatus  StatusInfo `json:"workload-status"`
	JujuStatus      StatusInfo `json:"juju-status"`
}

// StatusInfo holds status details.
type StatusInfo struct {
	Current string `json:"current"`
	Message string `json:"message"`
	Since   string `json:"since,omitempty"`
}

// StatusResult holds the full status check result for probe comparators.
type StatusResult struct {
	Current string `json:"current"`
	Message string `json:"message"`
	Since   string `json:"since,omitempty"`
}

// GetApplicationStatus returns the current status of a Juju application.
func GetApplicationStatus(ctx context.Context, client *common.JujuClient, app string) (*StatusResult, error) {
	var status JujuStatus
	if err := client.RunJSON(ctx, &status, "status", app); err != nil {
		return nil, fmt.Errorf("failed to get juju status: %w", err)
	}
	appStatus, ok := status.Applications[app]
	if !ok {
		return nil, fmt.Errorf("application %s not found in juju status", app)
	}
	return &StatusResult{
		Current: appStatus.ApplicationStatus.Current,
		Message: appStatus.ApplicationStatus.Message,
		Since:   appStatus.ApplicationStatus.Since,
	}, nil
}

// GetUnitStatus returns the workload status of a specific Juju unit.
func GetUnitStatus(ctx context.Context, client *common.JujuClient, unitName string) (*StatusResult, error) {
	// Extract app name from unit name (e.g. "mysql/0" -> "mysql")
	parts := strings.SplitN(unitName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid unit name %q: expected format app/N", unitName)
	}
	appName := parts[0]

	var status JujuStatus
	if err := client.RunJSON(ctx, &status, "status", appName); err != nil {
		return nil, fmt.Errorf("failed to get juju status: %w", err)
	}
	appStatus, ok := status.Applications[appName]
	if !ok {
		return nil, fmt.Errorf("application %s not found in juju status", appName)
	}
	unit, ok := appStatus.Units[unitName]
	if !ok {
		return nil, fmt.Errorf("unit %s not found in juju status", unitName)
	}
	return &StatusResult{
		Current: unit.WorkloadStatus.Current,
		Message: unit.WorkloadStatus.Message,
		Since:   unit.WorkloadStatus.Since,
	}, nil
}

// PebbleServiceInfo holds pebble service status from `pebble services --format=json`.
type PebbleServiceInfo struct {
	Name    string `json:"name"`
	Current string `json:"current"`
}

// GetPebbleServiceStatus returns the status of a pebble service on a Juju unit.
func GetPebbleServiceStatus(ctx context.Context, client *common.JujuClient, unitName, serviceName string) (*StatusResult, error) {
	output, err := client.Run(ctx, "ssh", unitName, "pebble", "services", serviceName, "--format=json")
	if err != nil {
		return nil, fmt.Errorf("failed to get pebble service status on %s: %w", unitName, err)
	}

	var services []PebbleServiceInfo
	if err := json.Unmarshal([]byte(output), &services); err != nil {
		return nil, fmt.Errorf("failed to parse pebble services output: %w", err)
	}

	for _, svc := range services {
		if svc.Name == serviceName {
			return &StatusResult{
				Current: svc.Current,
			}, nil
		}
	}
	return nil, fmt.Errorf("pebble service %q not found on unit %s", serviceName, unitName)
}

// WaitForApplicationStatus polls until the application reaches the target status or timeout.
func WaitForApplicationStatus(ctx context.Context, client *common.JujuClient, app, targetStatus string, timeout, delay int) error {
	log.Infof("[Juju] Waiting for application %s to reach status %q (timeout: %ds, delay: %ds)",
		app, targetStatus, timeout, delay)

	timeoutDuration := time.Duration(timeout) * time.Second
	delayDuration := time.Duration(delay) * time.Second
	deadline := time.Now().Add(timeoutDuration)

	for time.Now().Before(deadline) {
		var status JujuStatus
		err := client.RunJSON(ctx, &status, "status", app)
		if err != nil {
			log.Errorf("[Juju] Failed to get status: %v", err)
			time.Sleep(delayDuration)
			continue
		}

		appStatus, ok := status.Applications[app]
		if !ok {
			log.Infof("[Juju] Application %s not found in status yet", app)
			time.Sleep(delayDuration)
			continue
		}

		currentStatus := appStatus.ApplicationStatus.Current
		log.Infof("[Juju] Application %s current status: %s (target: %s)", app, currentStatus, targetStatus)

		if strings.EqualFold(currentStatus, targetStatus) {
			log.Infof("[Juju] Application %s reached target status %s", app, targetStatus)
			return nil
		}

		time.Sleep(delayDuration)
	}

	return fmt.Errorf("timeout waiting for application %s to reach status %q after %ds", app, targetStatus, timeout)
}
