package main

import (
	resourceapi "k8s.io/api/resource/v1beta1"
)

func enumerateAllPossibleDevices() (AllocatableDevices, error) {
	alldevices := make(AllocatableDevices)
	device := resourceapi.Device{Name: "dummy"}
	alldevices[device.Name] = device
	return alldevices, nil
}
