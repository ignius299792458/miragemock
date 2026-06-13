# Mirage Mock : Test Architecture

**Mirage Mock** is a layer-7 test architecture that copy live production traffic directly at the network level. Operating as a transparent proxy or sidecar, it intercepts raw HTTP byte streams copy its al background seperately from actual endpoint handlers. Using high-speed byte-boundary shifting, it surgically sanitizes sensitive values from payloads without altering structural keys, then copies the traffic asynchronously to staging (test or target env) servers.

# Architecture

1. First Time Scratched Architecture
   ![First Time Scratched Architecture](./docs/first_scratched_arch.png "First Time Scratched Architecture")
