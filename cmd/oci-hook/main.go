package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

type Hook struct {
	Spec *spec.Spec `json:"spec"`
}

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

	if err := mergo.Merge(&runtimeSpec, &runtimeSpecAdditions, mergo.WithOverride); err != nil {
		return fmt.Errorf("merging oci runtime spec: %w", err)
	}

	runtimeSpecData, err = json.Marshal(runtimeSpec)
	if err != nil {
		return fmt.Errorf("parsing current oci runtime spec: %v", err)
	}

	return os.WriteFile(runtimeSpecPath, runtimeSpecData, 0644)
}
