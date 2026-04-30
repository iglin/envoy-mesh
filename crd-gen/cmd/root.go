package cmd

import (
	"fmt"
	"os"

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
		return generator.Run(cfg)
	},
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
