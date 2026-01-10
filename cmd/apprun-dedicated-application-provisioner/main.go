package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/config"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/provisioner"
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

	Plan  PlanCmd  `cmd:"" help:"Show execution plan without making changes"`
	Apply ApplyCmd `cmd:"" help:"Apply the configuration changes"`
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
	Activate bool `help:"Activate the created/updated version after apply"`
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("apprun-provisioner"),
		kong.Description("Provision App Run Dedicated applications from YAML configuration"),
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

	p, err := createClient()
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

	p, err := createClient()
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

func createClient() (*provisioner.Provisioner, error) {
	accessToken := os.Getenv("SAKURA_ACCESS_TOKEN")
	accessTokenSecret := os.Getenv("SAKURA_ACCESS_TOKEN_SECRET")

	if accessToken == "" || accessTokenSecret == "" {
		return nil, fmt.Errorf("SAKURA_ACCESS_TOKEN and SAKURA_ACCESS_TOKEN_SECRET environment variables are required")
	}

	client, err := provisioner.NewClient(provisioner.ClientConfig{
		AccessToken:       accessToken,
		AccessTokenSecret: accessTokenSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return provisioner.NewProvisioner(client), nil
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

	fmt.Printf("\nPlan: %d to create, %d to update, %d unchanged.\n", createCount, updateCount, noopCount)
}
