package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/urfave/cli/v2"

	coreclientset "k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/klog/v2"

	"runtime-spec-dra-driver/pkg/flags"
)

const (
	DriverPluginCheckpointFile = "checkpoint.json"
)

type Flags struct {
	kubeClientConfig flags.KubeClientConfig
	loggingConfig    *flags.LoggingConfig

	nodeName                      string
	cdiRoot                       string
	numDevices                    int
	kubeletRegistrarDirectoryPath string
	kubeletPluginsDirectoryPath   string
	healthcheckPort               int
}

type Config struct {
	flags      *Flags
	coreclient coreclientset.Interface
}

func (c Config) DriverPluginPath() string {
	return filepath.Join(c.flags.kubeletPluginsDirectoryPath, DriverName)
}

func main() {
	if err := newApp().Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newApp() *cli.App {
	flags := &Flags{
		loggingConfig: flags.NewLoggingConfig(),
	}
	cliFlags := []cli.Flag{
		&cli.StringFlag{
			Name:        "node-name",
			Usage:       "The name of the node to be worked on.",
			Required:    true,
			Destination: &flags.nodeName,
			EnvVars:     []string{"NODE_NAME"},
		},
		&cli.StringFlag{
			Name:        "cdi-root",
			Usage:       "Absolute path to the directory where CDI files will be generated.",
			Value:       "/etc/cdi",
			Destination: &flags.cdiRoot,
			EnvVars:     []string{"CDI_ROOT"},
		},
		&cli.IntFlag{
			Name:        "num-devices",
			Usage:       "The number of devices to be generated.",
			Value:       8,
			Destination: &flags.numDevices,
			EnvVars:     []string{"NUM_DEVICES"},
		},
		&cli.StringFlag{
			Name:        "kubelet-registrar-directory-path",
			Usage:       "Absolute path to the directory where kubelet stores plugin registrations.",
			Value:       kubeletplugin.KubeletRegistryDir,
			Destination: &flags.kubeletRegistrarDirectoryPath,
			EnvVars:     []string{"KUBELET_REGISTRAR_DIRECTORY_PATH"},
		},
		&cli.StringFlag{
			Name:        "kubelet-plugins-directory-path",
			Usage:       "Absolute path to the directory where kubelet stores plugin data.",
			Value:       kubeletplugin.KubeletPluginsDir,
			Destination: &flags.kubeletPluginsDirectoryPath,
			EnvVars:     []string{"KUBELET_PLUGINS_DIRECTORY_PATH"},
		},
		&cli.IntFlag{
			Name:        "healthcheck-port",
			Usage:       "Port to start a gRPC healthcheck service. When positive, a literal port number. When zero, a random port is allocated. When negative, the healthcheck service is disabled.",
			Value:       -1,
			Destination: &flags.healthcheckPort,
			EnvVars:     []string{"HEALTHCHECK_PORT"},
		},
	}
	cliFlags = append(cliFlags, flags.kubeClientConfig.Flags()...)
	cliFlags = append(cliFlags, flags.loggingConfig.Flags()...)

	app := &cli.App{
		Name:            "dra-example-kubeletplugin",
		Usage:           "dra-example-kubeletplugin implements a DRA driver plugin.",
		ArgsUsage:       " ",
		HideHelpCommand: true,
		Flags:           cliFlags,
		Before: func(c *cli.Context) error {
			if c.Args().Len() > 0 {
				return fmt.Errorf("arguments not supported: %v", c.Args().Slice())
			}
			return flags.loggingConfig.Apply()
		},
		Action: func(c *cli.Context) error {
			ctx := c.Context
			clientSets, err := flags.kubeClientConfig.NewClientSets()
			if err != nil {
				return fmt.Errorf("create client: %v", err)
			}

			config := &Config{
				flags:      flags,
				coreclient: clientSets.Core,
			}

			return RunPlugin(ctx, config)
		},
	}

	return app
}

func RunPlugin(ctx context.Context, config *Config) error {
	logger := klog.FromContext(ctx)

	err := os.MkdirAll(config.DriverPluginPath(), 0750)
	if err != nil {
		return err
	}

	info, err := os.Stat(config.flags.cdiRoot)
	switch {
	case err != nil && os.IsNotExist(err):
		err := os.MkdirAll(config.flags.cdiRoot, 0750)
		if err != nil {
			return err
		}
	case err != nil:
		return err
	case !info.IsDir():
		return fmt.Errorf("path for cdi file generation is not a directory: '%v'", err)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	driver, err := NewDriver(ctx, config)
	if err != nil {
		return err
	}

	<-ctx.Done()
	// restore default signal behavior as soon as possible in case graceful
	// shutdown gets stuck.
	stop()
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		// A canceled context is the normal case here when the process receives
		// a signal. Only log the error for more interesting cases.
		logger.Error(err, "error from context")
	}

	err = driver.Shutdown(logger)
	if err != nil {
		logger.Error(err, "Unable to cleanly shutdown driver")
	}

	return nil
}
