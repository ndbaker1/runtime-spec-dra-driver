# E2E test targets
.PHONY: test-e2e test-e2e-setup test-e2e-run test-e2e-cleanup

CLUSTER_NAME ?= runtime-spec-dra-test

test-e2e: test-e2e-setup test-e2e-run test-e2e-cleanup

test-e2e-setup:
	./test/e2e/setup.sh

test-e2e-run:
	./test/e2e/run-tests.sh

test-e2e-cleanup:
	kind delete cluster --name=$(CLUSTER_NAME) 2>/dev/null || true
