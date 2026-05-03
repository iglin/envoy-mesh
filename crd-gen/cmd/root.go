package cmd

import (
	"fmt"
	"os"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/iglin/envoy-mesh/crd-gen/internal/generator"
	"github.com/spf13/cobra"
)

var cfg generator.Config

var rootCmd = &cobra.Command{
	Use:   "crd-gen",
	Short: "Generate Kubernetes CRDs from Envoy protobuf definitions",
	Example: `  # Generate CRDs for Listener and Cluster
  crd-gen -m envoy.config.listener.v3.Listener -m envoy.config.cluster.v3.Cluster

  # Custom paths and group
  crd-gen --proto-dir ./proto --out-dir ./crds --group mesh.example.io \
          -m envoy.config.listener.v3.Listener`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg.KindExtraProps = kindExtraProps()
		return generator.Run(cfg)
	},
}

// kindExtraProps returns additional top-level CRD schema properties that are
// Kubernetes-specific (not derived from Envoy proto) and must be injected per kind.
func kindExtraProps() map[string]map[string]apiextensionsv1.JSONSchemaProps {
	f64 := func(v float64) *float64 { return &v }
	return map[string]map[string]apiextensionsv1.JSONSchemaProps{
		"Cluster": {
			"kubernetesServiceRef": {
				Type:        "object",
				Description: "Auto-discovers endpoints from a Kubernetes Service and synthesises a ClusterLoadAssignment in memory. The cluster spec must use type: EDS with ads: {} as the eds_config source.",
				Required:    []string{"name"},
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"name":      {Type: "string", Description: "Name of the Kubernetes Service."},
					"namespace": {Type: "string", Description: "Namespace of the Service. Defaults to the Cluster CR's namespace."},
					"port": {
						Type:        "integer",
						Format:      "int32",
						Description: "Port to expose as Envoy endpoints. If omitted, the first port of the Service is used.",
						Minimum:     f64(1),
						Maximum:     f64(65535),
					},
				},
			},
		},
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	f := rootCmd.Flags()
	f.StringVar(&cfg.ProtoDir, "proto-dir", "../proto", "path to the proto source directory")
	f.StringVar(&cfg.OutDir, "out-dir", "../crds", "output directory for generated CRD YAMLs")
	f.StringVar(&cfg.Group, "group", "mesh.iglin.io", "CRD API group")
	f.StringVar(&cfg.Version, "version", "v1alpha1", "CRD API version")
	f.StringSliceVarP(&cfg.Messages, "message", "m", nil,
		"fully-qualified proto message name(s) to generate CRDs for\n"+
			"(e.g. envoy.config.listener.v3.Listener)")
	_ = rootCmd.MarkFlagRequired("message")
}
