# homebrew-exporter
A Snapcraft Exporter for parsing the metrics provided by Snapcraft: https://snapcraft.io/docs/snapcraft-metrics

### To configure

| ENV variable | Default value | Description |
|--------------|---------------|-------------|
| `METRICS_PATH` | `"/metrics"`| The path to publish the metrics to. |
| `LISTEN_PORT`  | `"9888"`    | The port the metrics exporter listens on. |
| `SNAP_NAMES` | REQUIRED | The list of Snaps to grab metrics for. If blank, the exporter will exit immediately. |
