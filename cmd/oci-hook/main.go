package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	runtimeSpecAdditionsData := os.Getenv("OCI_RUNTIME_SPEC")

	if runtimeSpecAdditionsData == "" {
		return nil
	}

	var runtimeSpecAdditions spec.Spec
	if err := json.Unmarshal([]byte(runtimeSpecAdditionsData), &runtimeSpecAdditions); err != nil {
		return fmt.Errorf("parsing user's oci runtime spec [%s]: %w ", runtimeSpecAdditionsData, err)
	}

	var state spec.State
	if err := json.NewDecoder(os.Stdin).Decode(&state); err != nil {
		return fmt.Errorf("parsing state: %w", err)
	}

	runtimeSpecPath := filepath.Join(state.Bundle, "config.json")

	runtimeSpecData, err := os.ReadFile(runtimeSpecPath)
	if err != nil {
		return fmt.Errorf("reading current oci runtime spec: %w", err)
	}

	var runtimeSpec spec.Spec
	if err := json.Unmarshal(runtimeSpecData, &runtimeSpec); err != nil {
		return fmt.Errorf("parsing current oci runtime spec: %v", err)
	}

	// TODO: this does not affect the container's actual cgroup config today
	// because merging the specs happens after the container config is loaded by
	// runc. this hook would if it were run whilst still in containerd's scope,
	// but that requires static, AoT configuration on the node. Since this
	// approach is trying to abuse the CDI `ContainerEdits.Hooks` field, we are
	// restricted by hooks inside runc rather than containerd.
	//
	// however, this hook approach will still work as long as we directly
	// configure cgroupv2, like writing the appropriate info into unified fields
	// such as io.max for example.

	if err := mergo.Merge(&runtimeSpec, &runtimeSpecAdditions, mergo.WithOverride); err != nil {
		return fmt.Errorf("merging oci runtime spec: %w", err)
	}

	runtimeSpecData, err = json.Marshal(runtimeSpec)
	if err != nil {
		return fmt.Errorf("parsing current oci runtime spec: %v", err)
	}

	return os.WriteFile(runtimeSpecPath, runtimeSpecData, 0644)
}
