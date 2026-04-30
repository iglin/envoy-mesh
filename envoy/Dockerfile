FROM envoyproxy/envoy:v1.33.2

# Bootstrap config is mounted from a ConfigMap at /etc/envoy/envoy.yaml.
# Envoy variable substitution (${VAR}) is enabled by default for values
# that reference environment variables set in the Deployment.

ENTRYPOINT ["/usr/local/bin/envoy"]
CMD ["-c", "/etc/envoy/envoy.yaml", "--log-level", "info"]
