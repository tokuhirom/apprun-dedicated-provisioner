package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/tokuhirom/apprun-dedicated-application-provisioner/config"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/provisioner"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/state"
)

// Version information (set by goreleaser)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type CLI struct {
	Config  string      `short:"c" help:"Path to config file"`
	Version VersionFlag `name:"version" help:"Print version information"`

	Plan     PlanCmd     `cmd:"" help:"Show execution plan without making changes"`
	Apply    ApplyCmd    `cmd:"" help:"Apply the configuration changes"`
	Versions VersionsCmd `cmd:"" help:"List application versions"`
	Diff     DiffCmd     `cmd:"" help:"Show diff between two versions"`
	Activate ActivateCmd `cmd:"" help:"Activate a version"`
	Dump     DumpCmd     `cmd:"" help:"Dump current cluster configuration as YAML"`
}

type VersionFlag bool

func (v VersionFlag) BeforeApply() error {
	fmt.Printf("apprun-dedicated-application-provisioner %s\n", version)
	fmt.Printf("  commit: %s\n", commit)
	fmt.Printf("  built:  %s\n", date)
	os.Exit(0)
	return nil
}

type PlanCmd struct{}

type ApplyCmd struct {
	Activate    bool `help:"Activate the created/updated version after apply"`
	AutoApprove bool `short:"y" name:"auto-approve" help:"Skip interactive approval of plan before applying"`
}

type VersionsCmd struct {
	App string `short:"a" help:"Application name" required:""`
}

type DiffCmd struct {
	App  string `short:"a" help:"Application name" required:""`
	From int    `help:"Source version (default: active version)" default:"0"`
	To   int    `help:"Target version (default: latest version)" default:"0"`
}

type ActivateCmd struct {
	App           string `short:"a" help:"Application name" required:""`
	TargetVersion int    `name:"target" short:"t" help:"Version to activate (default: latest)" default:"0"`
}

type DumpCmd struct {
	ClusterName string `arg:"" help:"Cluster name to dump"`
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("apprun-provisioner"),
		kong.Description("Provision AppRun Dedicated applications from YAML configuration"),
	)

	err := ctx.Run(&cli)
	ctx.FatalIfErrorf(err)
}

func (c *PlanCmd) Run(cli *CLI) error {
	if cli.Config == "" {
		return fmt.Errorf("--config (-c) is required")
	}
	cfg, err := loadConfig(cli.Config)
	if err != nil {
		return err
	}

	p, err := createProvisioner(cli.Config)
	if err != nil {
		return err
	}

	ctx := context.Background()
	plan, err := p.CreatePlan(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}

	printPlan(plan)
	return nil
}

func (c *ApplyCmd) Run(cli *CLI) error {
	if cli.Config == "" {
		return fmt.Errorf("--config (-c) is required")
	}
	cfg, err := loadConfig(cli.Config)
	if err != nil {
		return err
	}

	p, err := createProvisioner(cli.Config)
	if err != nil {
		return err
	}

	ctx := context.Background()

	plan, err := p.CreatePlan(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}

	printPlan(plan)

	hasChanges := false

	// Check for ASG changes (skip doesn't count as a change)
	for _, action := range plan.ASGActions {
		if action.Action != provisioner.ASGActionNoop && action.Action != provisioner.ASGActionSkip {
			hasChanges = true
			break
		}
	}

	// Check for LB changes (skip doesn't count as a change)
	for _, action := range plan.LBActions {
		if action.Action != provisioner.LBActionNoop && action.Action != provisioner.LBActionSkip {
			hasChanges = true
			break
		}
	}

	// Check for Application changes
	for _, action := range plan.Actions {
		if action.Action != provisioner.ActionNoop {
			hasChanges = true
			break
		}
	}

	if !hasChanges {
		fmt.Println("\nNo changes to apply.")
		return nil
	}

	// Prompt for confirmation unless --auto-approve is set
	if !c.AutoApprove {
		fmt.Print("\nDo you want to apply these changes? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Apply cancelled.")
			return nil
		}
	}

	fmt.Println("\nApplying changes...")

	opts := provisioner.ApplyOptions{
		Activate: c.Activate,
	}
	if err := p.Apply(ctx, cfg, plan, opts); err != nil {
		return fmt.Errorf("failed to apply plan: %w", err)
	}

	fmt.Println("\nApply complete!")
	return nil
}

func (c *VersionsCmd) Run(cli *CLI) error {
	if cli.Config == "" {
		return fmt.Errorf("--config (-c) is required")
	}
	cfg, err := loadConfig(cli.Config)
	if err != nil {
		return err
	}

	p, err := createProvisionerSimple()
	if err != nil {
		return err
	}

	ctx := context.Background()
	result, err := p.ListVersions(ctx, cfg.ClusterName, c.App)
	if err != nil {
		return fmt.Errorf("failed to list versions: %w", err)
	}

	printVersionList(result)
	return nil
}

func (c *DiffCmd) Run(cli *CLI) error {
	if cli.Config == "" {
		return fmt.Errorf("--config (-c) is required")
	}
	cfg, err := loadConfig(cli.Config)
	if err != nil {
		return err
	}

	p, err := createProvisionerSimple()
	if err != nil {
		return err
	}

	ctx := context.Background()
	diff, err := p.GetVersionDiff(ctx, cfg.ClusterName, c.App, c.From, c.To)
	if err != nil {
		return fmt.Errorf("failed to get version diff: %w", err)
	}

	printVersionDiff(c.App, diff)
	return nil
}

func (c *ActivateCmd) Run(cli *CLI) error {
	if cli.Config == "" {
		return fmt.Errorf("--config (-c) is required")
	}
	cfg, err := loadConfig(cli.Config)
	if err != nil {
		return err
	}

	p, err := createProvisionerSimple()
	if err != nil {
		return err
	}

	ctx := context.Background()
	activatedVersion, err := p.ActivateVersion(ctx, cfg.ClusterName, c.App, c.TargetVersion)
	if err != nil {
		return fmt.Errorf("failed to activate version: %w", err)
	}

	fmt.Printf("Successfully activated version %d for application %q\n", activatedVersion, c.App)
	return nil
}

func (c *DumpCmd) Run(cli *CLI) error {
	p, err := createProvisionerSimple()
	if err != nil {
		return err
	}

	ctx := context.Background()
	clusterConfig, err := p.DumpClusterConfig(ctx, c.ClusterName)
	if err != nil {
		return fmt.Errorf("failed to dump cluster config: %w", err)
	}

	// Output as YAML
	yamlOutput, err := clusterConfig.ToYAML()
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	fmt.Print(yamlOutput)
	return nil
}

func createProvisioner(configPath string) (*provisioner.Provisioner, error) {
	accessToken := getEnvWithFallback("SAKURA_ACCESS_TOKEN", "SAKURACLOUD_ACCESS_TOKEN")
	accessTokenSecret := getEnvWithFallback("SAKURA_ACCESS_TOKEN_SECRET", "SAKURACLOUD_ACCESS_TOKEN_SECRET")

	if accessToken == "" || accessTokenSecret == "" {
		return nil, fmt.Errorf("SAKURA_ACCESS_TOKEN (or SAKURACLOUD_ACCESS_TOKEN) and SAKURA_ACCESS_TOKEN_SECRET (or SAKURACLOUD_ACCESS_TOKEN_SECRET) environment variables are required")
	}

	client, err := provisioner.NewClient(provisioner.ClientConfig{
		AccessToken:       accessToken,
		AccessTokenSecret: accessTokenSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Load state file
	st, err := state.LoadState(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load state file: %w", err)
	}

	return provisioner.NewProvisioner(client, st, configPath), nil
}

func loadConfig(path string) (*config.ClusterConfig, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

func printPlan(plan *provisioner.Plan) {
	fmt.Printf("Cluster: %s (%s)\n\n", plan.ClusterName, plan.ClusterID)

	// Print ASG changes
	asgHasChanges := false
	for _, action := range plan.ASGActions {
		if action.Action != provisioner.ASGActionNoop && action.Action != provisioner.ASGActionSkip {
			asgHasChanges = true
			break
		}
	}
	if asgHasChanges || len(plan.ASGActions) > 0 {
		fmt.Println("=== Auto Scaling Groups ===")
		for _, action := range plan.ASGActions {
			switch action.Action {
			case provisioner.ASGActionCreate:
				fmt.Printf("+ %s (create)\n", action.Name)
				for _, change := range action.Changes {
					fmt.Printf("    %s\n", change)
				}
			case provisioner.ASGActionDelete:
				fmt.Printf("- %s (delete)\n", action.Name)
			case provisioner.ASGActionRecreate:
				fmt.Printf("~ %s (recreate - settings changed)\n", action.Name)
				for _, change := range action.Changes {
					fmt.Printf("    %s\n", change)
				}
			case provisioner.ASGActionSkip:
				fmt.Printf("  %s (not in YAML, skipping)\n", action.Name)
			case provisioner.ASGActionNoop:
				fmt.Printf("  %s (no changes)\n", action.Name)
			}
		}
		fmt.Println()
	}

	// Print LB changes
	lbHasChanges := false
	for _, action := range plan.LBActions {
		if action.Action != provisioner.LBActionNoop && action.Action != provisioner.LBActionSkip {
			lbHasChanges = true
			break
		}
	}
	if lbHasChanges || len(plan.LBActions) > 0 {
		fmt.Println("=== Load Balancers ===")
		for _, action := range plan.LBActions {
			switch action.Action {
			case provisioner.LBActionCreate:
				fmt.Printf("+ %s (create, ASG: %s)\n", action.Name, action.ASGName)
				for _, change := range action.Changes {
					fmt.Printf("    %s\n", change)
				}
			case provisioner.LBActionDelete:
				fmt.Printf("- %s (delete, ASG: %s)\n", action.Name, action.ASGName)
			case provisioner.LBActionRecreate:
				fmt.Printf("~ %s (recreate, ASG: %s - settings changed)\n", action.Name, action.ASGName)
				for _, change := range action.Changes {
					fmt.Printf("    %s\n", change)
				}
			case provisioner.LBActionSkip:
				fmt.Printf("  %s (not in YAML, skipping, ASG: %s)\n", action.Name, action.ASGName)
			case provisioner.LBActionNoop:
				fmt.Printf("  %s (no changes, ASG: %s)\n", action.Name, action.ASGName)
			}
		}
		fmt.Println()
	}

	// Print Application changes
	appHasChanges := false
	for _, action := range plan.Actions {
		if action.Action != provisioner.ActionNoop {
			appHasChanges = true
			break
		}
	}
	if appHasChanges || len(plan.Actions) > 0 {
		fmt.Println("=== Applications ===")
	}

	createCount := 0
	updateCount := 0
	noopCount := 0

	for _, action := range plan.Actions {
		switch action.Action {
		case provisioner.ActionCreate:
			createCount++
			fmt.Printf("+ %s (create)\n", action.ApplicationName)
			for _, change := range action.Changes {
				fmt.Printf("    %s\n", change)
			}
		case provisioner.ActionUpdate:
			updateCount++
			fmt.Printf("~ %s (update)\n", action.ApplicationName)
			for _, change := range action.Changes {
				fmt.Printf("    %s\n", change)
			}
		case provisioner.ActionNoop:
			noopCount++
			fmt.Printf("  %s (no changes)\n", action.ApplicationName)
		}
	}

	// Count infrastructure changes
	asgCreateCount, asgDeleteCount, asgRecreateCount := 0, 0, 0
	for _, action := range plan.ASGActions {
		switch action.Action {
		case provisioner.ASGActionCreate:
			asgCreateCount++
		case provisioner.ASGActionDelete:
			asgDeleteCount++
		case provisioner.ASGActionRecreate:
			asgRecreateCount++
		}
	}

	lbCreateCount, lbDeleteCount, lbRecreateCount := 0, 0, 0
	for _, action := range plan.LBActions {
		switch action.Action {
		case provisioner.LBActionCreate:
			lbCreateCount++
		case provisioner.LBActionDelete:
			lbDeleteCount++
		case provisioner.LBActionRecreate:
			lbRecreateCount++
		}
	}

	fmt.Printf("\nPlan Summary:\n")
	if asgCreateCount+asgDeleteCount+asgRecreateCount > 0 {
		fmt.Printf("  ASG: %d to create, %d to delete, %d to recreate\n", asgCreateCount, asgDeleteCount, asgRecreateCount)
	}
	if lbCreateCount+lbDeleteCount+lbRecreateCount > 0 {
		fmt.Printf("  LB: %d to create, %d to delete, %d to recreate\n", lbCreateCount, lbDeleteCount, lbRecreateCount)
	}
	fmt.Printf("  Applications: %d to create, %d to update, %d unchanged\n", createCount, updateCount, noopCount)
}

// getEnvWithFallback returns the value of the first environment variable that is set
func getEnvWithFallback(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

// createProvisionerSimple creates a provisioner without state file (for read-only operations)
func createProvisionerSimple() (*provisioner.Provisioner, error) {
	accessToken := getEnvWithFallback("SAKURA_ACCESS_TOKEN", "SAKURACLOUD_ACCESS_TOKEN")
	accessTokenSecret := getEnvWithFallback("SAKURA_ACCESS_TOKEN_SECRET", "SAKURACLOUD_ACCESS_TOKEN_SECRET")

	if accessToken == "" || accessTokenSecret == "" {
		return nil, fmt.Errorf("SAKURA_ACCESS_TOKEN (or SAKURACLOUD_ACCESS_TOKEN) and SAKURA_ACCESS_TOKEN_SECRET (or SAKURACLOUD_ACCESS_TOKEN_SECRET) environment variables are required")
	}

	client, err := provisioner.NewClient(provisioner.ClientConfig{
		AccessToken:       accessToken,
		AccessTokenSecret: accessTokenSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Use empty state for read-only operations
	st := state.NewState()

	return provisioner.NewProvisioner(client, st, ""), nil
}

func printVersionList(list *provisioner.VersionList) {
	fmt.Printf("Application: %s (%s)\n\n", list.ApplicationName, list.ApplicationID)

	if len(list.Versions) == 0 {
		fmt.Println("No versions found.")
		return
	}

	// Print header
	fmt.Printf("%-8s %-30s %-20s %-6s %s\n", "VERSION", "IMAGE", "CREATED", "NODES", "STATUS")

	// Sort versions by number (descending)
	// Note: API may return them in any order
	for i := len(list.Versions) - 1; i >= 0; i-- {
		v := list.Versions[i]
		status := ""
		if v.IsActive {
			status = "active"
		}
		fmt.Printf("%-8d %-30s %-20s %-6d %s\n",
			v.Version,
			truncateString(v.Image, 30),
			v.Created.Format("2006-01-02 15:04:05"),
			v.ActiveNodes,
			status,
		)
	}

	fmt.Printf("\nTotal: %d versions\n", len(list.Versions))
	if list.ActiveVersion > 0 {
		fmt.Printf("Active version: %d\n", list.ActiveVersion)
	} else {
		fmt.Println("Active version: (none)")
	}
	if list.LatestVersion > 0 {
		fmt.Printf("Latest version: %d\n", list.LatestVersion)
	}
}

func printVersionDiff(appName string, diff *provisioner.VersionDiff) {
	fmt.Printf("Application: %s\n", appName)
	fmt.Printf("Comparing version %d → %d\n\n", diff.FromVersion, diff.ToVersion)

	if len(diff.Changes) == 0 {
		fmt.Println("No differences found.")
	} else {
		for _, change := range diff.Changes {
			fmt.Printf("  %s\n", change)
		}
	}

	// Print warning about incomparable fields
	if diff.HasSecretEnv || diff.HasRegistryPwd {
		fmt.Println()
		fmt.Println("Note: secret env values and registryPassword cannot be compared (values not returned by API)")
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Keep the tail (tag part is usually more important for images)
	return "..." + s[len(s)-(maxLen-3):]
}
