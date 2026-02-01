package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"k8s.io/klog/v2"
)

const (
	// AnnotationKeyConfig is the annotation key for the OCI runtime spec config
	// The DRA plugin encodes the RuntimeSpecEditConfig.Spec as JSON in this annotation
	AnnotationKeyConfig = "nri.runtime-spec.io/config"

	// EnvKeyOCIRuntimeSpec is the environment variable key used by the DRA plugin
	// to pass the OCI runtime spec configuration via CDI container edits
	EnvKeyOCIRuntimeSpec = "OCI_RUNTIME_SPEC"
)

// Plugin implements the NRI plugin interface
type Plugin struct {
	stub stub.Stub
}

// Configure is called when the plugin is first registered with NRI
func (p *Plugin) Configure(_ context.Context, config, runtime, version string) (stub.EventMask, error) {
	klog.Infof("Configure called: runtime=%s, version=%s", runtime, version)

	// Subscribe to container creation events - this is where we can modify the container spec
	var mask stub.EventMask
	mask.Set(api.Event_CREATE_CONTAINER)
	return mask, nil
}

// Synchronize is called after Configure, providing the initial state of pods and containers
func (p *Plugin) Synchronize(_ context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	klog.Infof("Synchronize called: pods=%d, containers=%d", len(pods), len(containers))
	// No updates needed for existing containers
	return nil, nil
}

// Shutdown is called when NRI is shutting down the plugin
func (p *Plugin) Shutdown(_ context.Context) {
	klog.Info("Shutdown called")
}

// CreateContainer is called when a new container is being created
// This is where we apply the OCI runtime spec modifications from DRA claims
func (p *Plugin) CreateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	klog.V(2).Infof("CreateContainer called: pod=%s/%s, container=%s",
		pod.GetNamespace(), pod.GetName(), container.GetName())

	// Check for config in multiple sources (in order of precedence):
	// 1. Container annotations (nri.runtime-spec.io/config)
	// 2. Pod annotations (nri.runtime-spec.io/config)
	// 3. Container environment variable (OCI_RUNTIME_SPEC) - set by DRA plugin via CDI
	configJSON := getConfigAnnotation(pod, container)
	if configJSON == "" {
		configJSON = getConfigFromEnv(container)
	}
	if configJSON == "" {
		klog.V(3).Infof("No runtime-spec config found for container %s", container.GetName())
		return nil, nil, nil
	}

	klog.Infof("Found runtime-spec config for container %s in pod %s/%s", container.GetName(), pod.GetNamespace(), pod.GetName())

	// Parse the OCI runtime spec from the annotation
	var ociSpec spec.Spec
	if err := json.Unmarshal([]byte(configJSON), &ociSpec); err != nil {
		klog.Errorf("Failed to parse OCI runtime spec from annotation: %v", err)
		return nil, nil, fmt.Errorf("failed to parse OCI runtime spec: %w", err)
	}

	// Create container adjustment based on the OCI spec
	adjustment, err := createAdjustment(&ociSpec)
	if err != nil {
		klog.Errorf("Failed to create container adjustment: %v", err)
		return nil, nil, fmt.Errorf("failed to create container adjustment: %w", err)
	}

	if adjustment != nil {
		klog.Infof("Applying adjustments to container %s: unified=%v, env=%d, mounts=%d",
			container.GetName(),
			adjustment.GetLinux().GetResources().GetUnified(),
			len(adjustment.GetEnv()),
			len(adjustment.GetMounts()),
		)
	}

	return adjustment, nil, nil
}

// getConfigAnnotation retrieves the runtime-spec config annotation from pod or container
func getConfigAnnotation(pod *api.PodSandbox, container *api.Container) string {
	// First check container annotations
	if container.GetAnnotations() != nil {
		if config, ok := container.GetAnnotations()[AnnotationKeyConfig]; ok {
			return config
		}
	}

	// Fall back to pod annotations
	if pod.GetAnnotations() != nil {
		if config, ok := pod.GetAnnotations()[AnnotationKeyConfig]; ok {
			return config
		}
	}

	return ""
}

// getConfigFromEnv retrieves the runtime-spec config from container environment variables
// This is the mechanism used by the DRA plugin via CDI container edits
func getConfigFromEnv(container *api.Container) string {
	prefix := EnvKeyOCIRuntimeSpec + "="
	for _, env := range container.GetEnv() {
		if len(env) > len(prefix) && env[:len(prefix)] == prefix {
			return env[len(prefix):]
		}
	}
	return ""
}

// createAdjustment creates an NRI ContainerAdjustment from an OCI runtime spec
func createAdjustment(ociSpec *spec.Spec) (*api.ContainerAdjustment, error) {
	adjustment := &api.ContainerAdjustment{}
	hasAdjustments := false

	// TODO: i dont love having these manual implementations for merging, but
	// the types aren't super compatible so we have to live with it for now.

	// Apply Linux-specific adjustments
	if ociSpec.Linux != nil {
		linuxAdj := &api.LinuxContainerAdjustment{}

		if ociSpec.Linux.Resources != nil {
			resources := &api.LinuxResources{}

			// Apply unified cgroup v2 parameters
			if len(ociSpec.Linux.Resources.Unified) > 0 {
				resources.Unified = ociSpec.Linux.Resources.Unified
				hasAdjustments = true
				klog.V(2).Infof("Setting unified cgroup params: %v", ociSpec.Linux.Resources.Unified)
			}

			// Apply memory limits
			if ociSpec.Linux.Resources.Memory != nil {
				mem := ociSpec.Linux.Resources.Memory
				resources.Memory = &api.LinuxMemory{}
				if mem.Limit != nil {
					resources.Memory.Limit = &api.OptionalInt64{Value: *mem.Limit}
					hasAdjustments = true
				}
				if mem.Reservation != nil {
					resources.Memory.Reservation = &api.OptionalInt64{Value: *mem.Reservation}
					hasAdjustments = true
				}
				if mem.Swap != nil {
					resources.Memory.Swap = &api.OptionalInt64{Value: *mem.Swap}
					hasAdjustments = true
				}
				if mem.Swappiness != nil {
					resources.Memory.Swappiness = &api.OptionalUInt64{Value: *mem.Swappiness}
					hasAdjustments = true
				}
				if mem.DisableOOMKiller != nil {
					resources.Memory.DisableOomKiller = &api.OptionalBool{Value: *mem.DisableOOMKiller}
					hasAdjustments = true
				}
			}

			// Apply CPU limits
			if ociSpec.Linux.Resources.CPU != nil {
				cpu := ociSpec.Linux.Resources.CPU
				resources.Cpu = &api.LinuxCPU{}
				if cpu.Shares != nil {
					resources.Cpu.Shares = &api.OptionalUInt64{Value: *cpu.Shares}
					hasAdjustments = true
				}
				if cpu.Quota != nil {
					resources.Cpu.Quota = &api.OptionalInt64{Value: *cpu.Quota}
					hasAdjustments = true
				}
				if cpu.Period != nil {
					resources.Cpu.Period = &api.OptionalUInt64{Value: *cpu.Period}
					hasAdjustments = true
				}
				if cpu.Cpus != "" {
					resources.Cpu.Cpus = cpu.Cpus
					hasAdjustments = true
				}
				if cpu.Mems != "" {
					resources.Cpu.Mems = cpu.Mems
					hasAdjustments = true
				}
			}

			// Apply hugepage limits
			if len(ociSpec.Linux.Resources.HugepageLimits) > 0 {
				for _, hp := range ociSpec.Linux.Resources.HugepageLimits {
					resources.HugepageLimits = append(resources.HugepageLimits, &api.HugepageLimit{
						PageSize: hp.Pagesize,
						Limit:    hp.Limit,
					})
				}
				hasAdjustments = true
			}

			linuxAdj.Resources = resources
		}

		adjustment.Linux = linuxAdj
	}

	// Apply environment variables
	if ociSpec.Process != nil && len(ociSpec.Process.Env) > 0 {
		for _, env := range ociSpec.Process.Env {
			adjustment.Env = append(adjustment.Env, &api.KeyValue{
				Key: env,
				// NRI expects key=value format in Key field, so the Value field is a noop.
				Value: "",
			})
		}
		hasAdjustments = true
		klog.V(2).Infof("Adding %d environment variables", len(ociSpec.Process.Env))
	}

	// Apply mounts
	if len(ociSpec.Mounts) > 0 {
		for _, m := range ociSpec.Mounts {
			adjustment.Mounts = append(adjustment.Mounts, &api.Mount{
				Destination: m.Destination,
				Type:        m.Type,
				Source:      m.Source,
				Options:     m.Options,
			})
		}
		hasAdjustments = true
		klog.V(2).Infof("Adding %d mounts", len(ociSpec.Mounts))
	}

	// Apply OCI hooks
	if ociSpec.Hooks != nil {
		hooks := convertHooks(ociSpec.Hooks)
		if hooks != nil {
			adjustment.Hooks = hooks
			hasAdjustments = true
			klog.V(2).Infof("Adding OCI hooks")
		}
	}

	// Apply Linux devices
	if ociSpec.Linux != nil && len(ociSpec.Linux.Devices) > 0 {
		for _, d := range ociSpec.Linux.Devices {
			dev := &api.LinuxDevice{
				Path:  d.Path,
				Type:  d.Type,
				Major: d.Major,
				Minor: d.Minor,
			}
			if d.FileMode != nil {
				dev.FileMode = &api.OptionalFileMode{Value: uint32(*d.FileMode)}
			}
			if d.UID != nil {
				dev.Uid = &api.OptionalUInt32{Value: *d.UID}
			}
			if d.GID != nil {
				dev.Gid = &api.OptionalUInt32{Value: *d.GID}
			}
			adjustment.Linux.Devices = append(adjustment.Linux.Devices, dev)
		}
		hasAdjustments = true
		klog.V(2).Infof("Adding %d Linux devices", len(ociSpec.Linux.Devices))
	}

	if !hasAdjustments {
		return nil, nil
	}

	return adjustment, nil
}

// convertHooks converts OCI hooks to NRI hooks
func convertHooks(ociHooks *spec.Hooks) *api.Hooks {
	if ociHooks == nil {
		return nil
	}

	hooks := &api.Hooks{}
	hasHooks := false

	if len(ociHooks.Prestart) > 0 {
		for _, h := range ociHooks.Prestart {
			hooks.Prestart = append(hooks.Prestart, convertHook(h))
		}
		hasHooks = true
	}

	if len(ociHooks.CreateRuntime) > 0 {
		for _, h := range ociHooks.CreateRuntime {
			hooks.CreateRuntime = append(hooks.CreateRuntime, convertHook(h))
		}
		hasHooks = true
	}

	if len(ociHooks.CreateContainer) > 0 {
		for _, h := range ociHooks.CreateContainer {
			hooks.CreateContainer = append(hooks.CreateContainer, convertHook(h))
		}
		hasHooks = true
	}

	if len(ociHooks.StartContainer) > 0 {
		for _, h := range ociHooks.StartContainer {
			hooks.StartContainer = append(hooks.StartContainer, convertHook(h))
		}
		hasHooks = true
	}

	if len(ociHooks.Poststart) > 0 {
		for _, h := range ociHooks.Poststart {
			hooks.Poststart = append(hooks.Poststart, convertHook(h))
		}
		hasHooks = true
	}

	if len(ociHooks.Poststop) > 0 {
		for _, h := range ociHooks.Poststop {
			hooks.Poststop = append(hooks.Poststop, convertHook(h))
		}
		hasHooks = true
	}

	if !hasHooks {
		return nil
	}

	return hooks
}

// convertHook converts an OCI hook to an NRI hook
func convertHook(h spec.Hook) *api.Hook {
	hook := &api.Hook{
		Path: h.Path,
		Args: h.Args,
		Env:  h.Env,
	}
	if h.Timeout != nil {
		hook.Timeout = &api.OptionalInt{Value: int64(*h.Timeout)}
	}
	return hook
}
