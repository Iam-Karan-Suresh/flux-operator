// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package kubeconfig

import (
	"encoding/base64"
	"fmt"

	"sigs.k8s.io/yaml"
)

// KubeConfig represents the minimal structure needed to extract
// API server address and CA certificate from a kubeconfig.
type KubeConfig struct {
	Clusters []Cluster `yaml:"clusters"`
}

// Cluster represents a cluster entry in the kubeconfig.
type Cluster struct {
	Name    string        `yaml:"name"`
	Cluster ClusterConfig `yaml:"cluster"`
}

// ClusterConfig contains the cluster configuration details.
type ClusterConfig struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
}

// ExtractFluxFields parses a kubeconfig YAML and extracts the fields
// needed for Flux workload identity ConfigMap (server and CA certificate).
// It returns the API server endpoint, the decoded CA certificate in PEM format,
// and any error encountered during parsing.
//
// The function extracts data from the first cluster in the kubeconfig.
// The CA certificate is base64-decoded from the certificate-authority-data field.
func ExtractFluxFields(kubeconfigYAML string) (server, caCert string, err error) {
	var config KubeConfig
	if err := yaml.Unmarshal([]byte(kubeconfigYAML), &config); err != nil {
		return "", "", fmt.Errorf("failed to parse kubeconfig YAML: %w", err)
	}

	if len(config.Clusters) == 0 {
		return "", "", fmt.Errorf("no clusters found in kubeconfig")
	}

	// Use the first cluster for now
	// TODO: Support cluster selection by name in future enhancement
	cluster := config.Clusters[0].Cluster

	if cluster.Server == "" {
		return "", "", fmt.Errorf("server field is empty in kubeconfig cluster")
	}

	if cluster.CertificateAuthorityData == "" {
		return "", "", fmt.Errorf("certificate-authority-data field is empty in kubeconfig cluster")
	}

	server = cluster.Server

	// Decode base64 CA certificate to PEM format
	caBytes, err := base64.StdEncoding.DecodeString(cluster.CertificateAuthorityData)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode certificate-authority-data: %w", err)
	}
	caCert = string(caBytes)

	return server, caCert, nil
}
