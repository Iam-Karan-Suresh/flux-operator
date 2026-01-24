// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package kubeconfig

import (
	"testing"
)

func TestExtractFluxFields(t *testing.T) {
	tests := []struct {
		name           string
		kubeconfigYAML string
		expectedServer string
		expectedCACert string
		expectError    bool
		errorContains  string
	}{
		{
			name: "valid CAPI kubeconfig",
			kubeconfigYAML: `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0ZXN0MTIzCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
    server: https://172.18.0.3:6443
  name: capi-helloworld
contexts:
- context:
    cluster: capi-helloworld
    user: capi-helloworld-admin
  name: capi-helloworld-admin@capi-helloworld
current-context: capi-helloworld-admin@capi-helloworld
kind: Config
preferences: {}
users:
- name: capi-helloworld-admin
  user:
    client-certificate-data: LS0tLS1...
    client-key-data: LS0tLS1...`,
			expectedServer: "https://172.18.0.3:6443",
			expectedCACert: `-----BEGIN CERTIFICATE-----
MIICtest123
-----END CERTIFICATE-----`,
			expectError: false,
		},
		{
			name: "multiple clusters - uses first",
			kubeconfigYAML: `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0ZXN0MTIzCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
    server: https://first-cluster:6443
  name: first-cluster
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0ZXN0MTIzCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
    server: https://second-cluster:6443
  name: second-cluster`,
			expectedServer: "https://first-cluster:6443",
			expectedCACert: `-----BEGIN CERTIFICATE-----
MIICtest123
-----END CERTIFICATE-----`,
			expectError: false,
		},
		{
			name:           "invalid YAML",
			kubeconfigYAML: `this is not valid yaml: [`,
			expectError:    true,
			errorContains:  "failed to parse kubeconfig YAML",
		},
		{
			name: "no clusters",
			kubeconfigYAML: `apiVersion: v1
clusters: []`,
			expectError:   true,
			errorContains: "no clusters found in kubeconfig",
		},
		{
			name: "missing server field",
			kubeconfigYAML: `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0ZXN0MTIzCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
  name: test-cluster`,
			expectError:   true,
			errorContains: "server field is empty",
		},
		{
			name: "missing CA data",
			kubeconfigYAML: `apiVersion: v1
clusters:
- cluster:
    server: https://test-cluster:6443
  name: test-cluster`,
			expectError:   true,
			errorContains: "certificate-authority-data field is empty",
		},
		{
			name: "invalid base64 CA data",
			kubeconfigYAML: `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: "not-valid-base64!!!"
    server: https://test-cluster:6443
  name: test-cluster`,
			expectError:   true,
			errorContains: "failed to decode certificate-authority-data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, caCert, err := ExtractFluxFields(tt.kubeconfigYAML)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" {
					if err.Error() != tt.errorContains && len(err.Error()) > 0 {
						// Check if error contains the substring
						found := false
						for i := 0; i <= len(err.Error())-len(tt.errorContains); i++ {
							if err.Error()[i:i+len(tt.errorContains)] == tt.errorContains {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("expected error to contain %q, got: %v", tt.errorContains, err)
						}
					}
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if server != tt.expectedServer {
				t.Errorf("expected server %q, got %q", tt.expectedServer, server)
			}

			if caCert != tt.expectedCACert {
				t.Errorf("expected CA cert %q, got %q", tt.expectedCACert, caCert)
			}
		})
	}
}
