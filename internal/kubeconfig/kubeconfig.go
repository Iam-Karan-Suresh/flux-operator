// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package kubeconfig

import (
	"encoding/base64"
	"fmt"

	"sigs.k8s.io/yaml"
)

type KubeConfig struct {
	Clusters []Cluster `yaml:"clusters" json:"clusters"`
}
type Cluster struct {
	Name    string        `yaml:"name" json:"name"`
	Cluster ClusterConfig `yaml:"cluster" json:"cluster"`
}
type ClusterConfig struct {
	Server                   string `yaml:"server" json:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty" json:"certificate-authority-data,omitempty"`
}

type ClusterData struct {
	Name string
	Server string
	CACert string
}

func ExtractAllFluxFields(kubeconfigYAML string) ([]ClusterData, error) {
	var config KubeConfig
	if err := yaml.Unmarshal([]byte(kubeconfigYAML), &config); err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig YAML: %w", err)
	}

	if len(config.Clusters) == 0 {
		return nil, fmt.Errorf("no clusters found in kubeconfig")
	}

	clusters := make([]ClusterData, 0, len(config.Clusters))
	for _, c := range config.Clusters {
		cluster := c.Cluster

		if cluster.Server == "" {
			return nil, fmt.Errorf("server field is empty in kubeconfig cluster %q", c.Name)
		}

		if cluster.CertificateAuthorityData == "" {
			return nil, fmt.Errorf("certificate-authority-data field is empty in kubeconfig cluster %q", c.Name)
		}

		caBytes, err := base64.StdEncoding.DecodeString(cluster.CertificateAuthorityData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode certificate-authority-data for cluster %q: %w", c.Name, err)
		}

		clusters = append(clusters, ClusterData{
			Name:   c.Name,
			Server: cluster.Server,
			CACert: string(caBytes),
		})
	}

	return clusters, nil
}

func ExtractFluxFields(kubeconfigYAML string) (server, caCert string, err error) {
	clusters, err := ExtractAllFluxFields(kubeconfigYAML)
	if err != nil {
		return "", "", err
	}

	return clusters[0].Server, clusters[0].CACert, nil
}