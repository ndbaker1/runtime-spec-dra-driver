package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path"
	"strconv"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	drapb "k8s.io/kubelet/pkg/apis/dra/v1beta1"
	registerapi "k8s.io/kubelet/pkg/apis/pluginregistration/v1"
)

type healthcheck struct {
	grpc_health_v1.UnimplementedHealthServer

	server *grpc.Server
	wg     sync.WaitGroup

	regClient registerapi.RegistrationClient
	draClient drapb.DRAPluginClient
}

func startHealthcheck(ctx context.Context, config *Config) (*healthcheck, error) {
	log := klog.FromContext(ctx)

	port := config.flags.healthcheckPort
	if port < 0 {
		return nil, nil
	}

	addr := net.JoinHostPort("", strconv.Itoa(port))
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen for healthcheck service at %s: %w", addr, err)
	}

	regSockPath := (&url.URL{
		Scheme: "unix",
		// TODO: this needs to adapt when seamless upgrades
		// are enabled and the filename includes a uid.
		Path: path.Join(config.flags.kubeletRegistrarDirectoryPath, DriverName+"-reg.sock"),
	}).String()
	log.Info("connecting to registration socket", "path", regSockPath)
	regConn, err := grpc.NewClient(
		regSockPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to registration socket: %w", err)
	}

	draSockPath := (&url.URL{
		Scheme: "unix",
		Path:   path.Join(config.DriverPluginPath(), "dra.sock"),
	}).String()
	log.Info("connecting to DRA socket", "path", draSockPath)
	draConn, err := grpc.NewClient(
		draSockPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to DRA socket: %w", err)
	}

	server := grpc.NewServer()
	healthcheck := &healthcheck{
		server:    server,
		regClient: registerapi.NewRegistrationClient(regConn),
		draClient: drapb.NewDRAPluginClient(draConn),
	}
	grpc_health_v1.RegisterHealthServer(server, healthcheck)

	healthcheck.wg.Add(1)
	go func() {
		defer healthcheck.wg.Done()
		log.Info("starting healthcheck service", "addr", lis.Addr().String())
		if err := server.Serve(lis); err != nil {
			log.Error(err, "failed to serve healthcheck service", "addr", addr)
		}
	}()

	return healthcheck, nil
}

func (h *healthcheck) Stop(logger klog.Logger) {
	if h.server != nil {
		logger.Info("stopping healthcheck service")
		h.server.GracefulStop()
	}
	h.wg.Wait()
}

// Check implements [grpc_health_v1.HealthServer].
func (h *healthcheck) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	log := klog.FromContext(ctx)

	knownServices := map[string]struct{}{"": {}, "liveness": {}}
	if _, known := knownServices[req.GetService()]; !known {
		return nil, status.Error(codes.NotFound, "unknown service")
	}

	status := &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
	}

	info, err := h.regClient.GetInfo(ctx, &registerapi.InfoRequest{})
	if err != nil {
		log.Error(err, "failed to call GetInfo")
		return status, nil
	}
	log.V(5).Info("Successfully invoked GetInfo", "info", info)

	_, err = h.draClient.NodePrepareResources(ctx, &drapb.NodePrepareResourcesRequest{})
	if err != nil {
		log.Error(err, "failed to call NodePrepareResources")
		return status, nil
	}
	log.V(5).Info("Successfully invoked NodePrepareResources")

	status.Status = grpc_health_v1.HealthCheckResponse_SERVING
	return status, nil
}
