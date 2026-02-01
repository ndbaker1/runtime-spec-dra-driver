package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/containerd/nri/pkg/stub"
	"k8s.io/klog/v2"
)

const (
	// PluginName is the name of this NRI plugin
	PluginName = "runtime-spec-dra"
	// PluginIdx is the index of this plugin (determines order of execution)
	PluginIdx = "10"
)

var (
	version = "dev"
)

func main() {
	var (
		pluginName string
		pluginIdx  string
		socketPath string
	)

	flag.StringVar(&pluginName, "name", PluginName, "plugin name to register with NRI")
	flag.StringVar(&pluginIdx, "idx", PluginIdx, "plugin index to register with NRI")
	flag.StringVar(&socketPath, "socket", "", "NRI socket path to connect to")

	klog.InitFlags(nil)
	flag.Parse()

	klog.Infof("Starting %s NRI plugin version %s", pluginName, version)

	plugin := &Plugin{}

	opts := []stub.Option{
		stub.WithPluginName(pluginName),
		stub.WithPluginIdx(pluginIdx),
	}
	if socketPath != "" {
		opts = append(opts, stub.WithSocketPath(socketPath))
	}

	var err error
	plugin.stub, err = stub.New(plugin, opts...)
	if err != nil {
		klog.Fatalf("Failed to create NRI stub: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		klog.Infof("Received signal %v, shutting down", sig)
		cancel()
	}()

	err = plugin.stub.Run(ctx)
	if err != nil {
		klog.Fatalf("Plugin exited with error: %v", err)
	}

	klog.Info("Plugin exited successfully")
}

func printVersion() {
	fmt.Printf("nri-plugin version %s\n", version)
}
