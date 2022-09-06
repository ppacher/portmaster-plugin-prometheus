package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/ppacher/portmaster-plugin-prometheus/promreport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/safing/portmaster/plugin/framework"
	"github.com/safing/portmaster/plugin/framework/cmds"
	"github.com/safing/portmaster/plugin/shared"
	"github.com/safing/portmaster/plugin/shared/proto"
	"github.com/spf13/cobra"
)

type Mode string

const (
	ModeDefault = "" // pull
	ModePull    = "pull"
	ModePush    = "push"
)

type PushConfig struct {
	Interval time.Duration `json:"interval"`
	JobName  string        `json:"job"`
}

type StaticConfig struct {
	Namespace  string      `json:"namespace"`
	Subsystem  string      `json:"subsystem"`
	Address    string      `json:"listenAddress"`
	Mode       Mode        `json:"mode"`
	PushConfig *PushConfig `json:"push,omitempty"`
}

func startPlugin() {
	var reporter *promreport.PrometheusReporter

	framework.RegisterReporter(
		// we wrap the PromethesuReporter in a ReporterFunc so we can lazily initialize
		// the reporter during framework.OnInit.
		// We need this because access to the static configuration is only possible after
		// the framework has been initialized and this configuration is needed for
		// subsystem and namespace values.
		//
		// TODO(ppacher): We don't support changing this values during runtime (using framework.Config())
		// yet.
		framework.ReporterFunc(func(ctx context.Context, c *proto.Connection) error {
			// we ignore all connections that are from this plugin so we don't create
			// an endless loop here.
			if self, _ := framework.IsSelf(c); self {
				return nil
			}

			// reporter must be non-nil here because otherwise OnInit would have returned
			// an error and the plugin would not be used at all.
			return reporter.ReportConnection(ctx, c)
		}),
	)

	framework.OnInit(func(ctx context.Context) error {
		// Parse the static plugin configuration that is given in the plugins.json file.
		var (
			cfg StaticConfig
			err error
		)
		if err := framework.ParseStaticConfig(&cfg); err != nil {
			return fmt.Errorf("failed to parse required static configuration: %w", err)
		}

		// Create and initialize the prometheus reporter.
		reporter, err = promreport.NewPrometheusReporter(&promreport.Config{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
		})
		if err != nil {
			return fmt.Errorf("failed to create prometheus reporter: %w", err)
		}

		// finally, depending on the mode of operation, we need to either setup
		// a http server or configure a pushgateway.
		switch cfg.Mode {
		case ModeDefault, ModePull:
			err = setupHTTPServer(cfg.Address)
		case ModePush:
			err = setupPushgateway(cfg.Address, cfg.PushConfig)
		default:
			return fmt.Errorf("invalid operation mode %q, valid values are 'push' or 'pull'", cfg.Mode)
		}

		if err != nil {
			return err
		}

		return nil
	})

	framework.Serve()
}

func setupHTTPServer(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := http.Server{
		Addr: address,
		BaseContext: func(l net.Listener) context.Context {
			return framework.Context()
		},
		Handler: mux,
	}

	ch := make(chan error, 1)
	go func() {
		if err := srv.Serve(listener); err != nil {
			ch <- err
		}
	}()

	// give it some time for the server to startup
	select {
	case err := <-ch:
		return err
	case <-time.After(time.Millisecond * 500):
		return nil
	}
}

func setupPushgateway(address string, cfg *PushConfig) error {
	if cfg == nil {
		cfg = &PushConfig{}
	}

	if cfg.Interval == 0 {
		cfg.Interval = time.Second * 10
	}

	if cfg.JobName == "" {
		cfg.JobName = "portmaster"
	}

	pusher := push.New(address, cfg.JobName).Gatherer(prometheus.DefaultGatherer)

	go func() {
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := pusher.PushContext(framework.Context()); err != nil {
					hclog.L().Error("failed to push metrics", "error", err)
				}
			case <-framework.Context().Done():
				return
			}
		}
	}()

	return nil
}

func rootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "portmaster-plugin-prometheus",
		Run: func(cmd *cobra.Command, args []string) {
			startPlugin()
		},
	}

	return cmd
}

func main() {
	cmd := rootCommand()

	installConfig := &cmds.InstallCommandConfig{
		PluginName: "portmaster-plugin-prometheus",
		Types: []shared.PluginType{
			shared.PluginTypeReporter,
		},
	}

	installCommand := cmds.InstallCommand(installConfig)

	// We add some flags to the InstallCommand and use a PreRun function to configure
	// the static configuration in the installConfig so the InstallCommand() will
	// write that out as well.
	cfg := StaticConfig{
		PushConfig: &PushConfig{},
	}
	flags := installCommand.Flags()
	{
		flags.StringVar(&cfg.Namespace, "namespace", "", "The namespace for the prometheus metrics")
		flags.StringVar(&cfg.Subsystem, "subsystem", "", "The subsystem for the prometheus metrics")
		flags.StringVar(&cfg.Address, "address", "0.0.0.0:8081", "The listen address for pull mode and the address of the pushgateway in push mode")
		flags.StringVar((*string)(&cfg.Mode), "mode", "pull", "The operation mode. Either pull or push")
		flags.DurationVar(&cfg.PushConfig.Interval, "push-interval", 0, "The interval between pushes to the gateway")
		flags.StringVar(&cfg.PushConfig.JobName, "push-job-name", "portmaster", "The name of the job when pushing to the gateway")
	}

	// setup a pre-run function that will update installConfig.StaticConfig according to the
	// flags above.
	installCommand.PreRun = func(cmd *cobra.Command, args []string) {
		switch cfg.Mode {
		case ModePull:
			cfg.PushConfig = nil
		case ModePush:
		default:
			hclog.L().Error("invalid value for --mode", "mode", cfg.Mode)
			os.Exit(1)
		}

		blob, err := json.MarshalIndent(cfg, "", "    ")
		if err != nil {
			hclog.L().Error("failed to marshal static configuration", "error", err)
			os.Exit(1)
		}

		// Update the install configuration to contain the static config as well.
		installConfig.StaticConfig = blob
	}

	// add it to the root command
	cmd.AddCommand(installCommand)

	// and finally run the plugin
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
