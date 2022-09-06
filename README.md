# `portmaster-plugin-prometheus`

This repository provides a [Safing Portmaster](https://github.com/safing/portmaster) plugin that exposes connection statistics to Prometheus. It supports pull (scraping) and push mode.

**Warning**: This repository is based on the experimental Portmaster Plugin System which is available in [safing/portmaster#834](https://github.com/safing/portmaster/pull/834) but has not been merged and released yet.

## Installation

### Manually 

To manually install the plugin follow these steps:

1. Build the plugin from source code: `go build .`
2. Move the plugin `/opt/safing/portmaster/plugins/portmaster-plugin-prometheus`
3. Edit `/opt/safing/portmaster/plugins.json` to contain the following content:

   ```
   [
        {
            "name": "portmaster-plugin-prometheus",
            "types": [
                "decider"
            ],
            "config": {
                "namespace": "",
                "subsystem": "",
                "mode": "pull",
                "address": "0.0.0.0:8081",
                "push": {
                    "interval": "10s",
                    "jobName": "portmaster"
                }
            }
        }
   ]
   ```

### Using the install command

This plugin uses the `cmds.InstallCommand()` from the portmaster plugin framework so installation is as simple as:

```bash
go build .
sudo ./portmaster-plugin-prometheus install --data /opt/safing/portmaster
```

## Configuration

**Important**: Before being able to use plugins in the Portmaster you must enable the "Plugin System" in the global settings page. Note that this setting is still marked as "Experimental" and "Developer-Only" so you'r Portmaster needs the following settings adjusted to even show the "Plugin System" setting:

 - [Developer Mode](https://docs.safing.io/portmaster/settings#core/devMode)
 - [Feature Stability](https://docs.safing.io/portmaster/settings#core/releaseLevel)

The plugin can either be configured using static configuration in `plugins.json`.

Just specify the static configuration when using `portmaster-plugin-prometheus install`. For example:

```bash
sudo ./portmaster-plugin-prometheus install --mode push --address https://pushgateway.example.com/metrics/ --push-job-name portmaster --push-interval 10s --data /opt/safing/portmaster
```