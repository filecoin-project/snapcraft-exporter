package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type SnapcraftMetricsItem struct {
	Name     string `json:"name"`
	Values   []int  `json:"values"`
	Released bool   `json:"currently_released,omitempty"`
}

type SnapcraftMetrics struct {
	Buckets    []string               `json:"buckets"`
	MetricName string                 `json:"metric_name"`
	Series     []SnapcraftMetricsItem `json:"series"`
	SnapID     string                 `json:"snap_id"`
	Status     string                 `json:"status"`
}

type SnapcraftCollector struct {
	SnapNames                  []string
	deviceChangeDaily          *prometheus.Desc
	deviceChangeWeekly         *prometheus.Desc
	installBaseByChannelDaily  *prometheus.Desc
	installBaseByCountryDaily  *prometheus.Desc
	installBaseBySystemDaily   *prometheus.Desc
	installBaseByVersionDaily  *prometheus.Desc
	installBaseByChannelWeekly *prometheus.Desc
	installBaseByCountryWeekly *prometheus.Desc
	installBaseBySystemWeekly  *prometheus.Desc
	installBaseByVersionWeekly *prometheus.Desc
}

func newSnapcraftCollector(snapNames []string) *SnapcraftCollector {
	return &SnapcraftCollector{
		SnapNames: snapNames,
		deviceChangeDaily: prometheus.NewDesc("snapcraft_device_change_daily",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. daily_device_change: contains the 3 series representing the number of new, continued and lost devices with the given snap installed compared to the previous day.",
			[]string{"snap", "change"}, nil,
		),
		deviceChangeWeekly: prometheus.NewDesc("snapcraft_device_change_weekly",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. weekly_device_change: similar to the ‘daily_device_change’ metric but operates on a 7 day window. i.e. new contains the number of devices that were seen during the last 7 days but not in the previous 7 day and so on for continued and lost.",
			[]string{"snap", "change"}, nil,
		),
		installBaseByChannelDaily: prometheus.NewDesc("snapcraft_install_base_by_channel_daily",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. installed_base_by_channel: contains one series per channel representing the number of devices with the given snap installed, channels with no data across the entire interval are omitted.",
			[]string{"snap", "channel"}, nil,
		),
		installBaseByCountryDaily: prometheus.NewDesc("snapcraft_install_base_by_country_daily",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. installed_base_by_country: contains one series per country representing the number of devices with the given snap installed.",
			[]string{"snap", "country"}, nil,
		),
		installBaseBySystemDaily: prometheus.NewDesc("snapcraft_install_base_by_system_daily",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. installed_base_by_operating_system: contains one series per operating_system representing the number of devices with the given snap installed.",
			[]string{"snap", "system"}, nil,
		),
		installBaseByVersionDaily: prometheus.NewDesc("snapcraft_install_base_by_version_daily",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. installed_base_by_version: contains one series per version representing the number of devices with the given snap installed.",
			[]string{"snap", "version"}, nil,
		),
		installBaseByChannelWeekly: prometheus.NewDesc("snapcraft_install_base_by_channel_weekly",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. weekly_installed_base_by_channel: similar to the installed_base_by_channel metric but operates in a 7 day window.",
			[]string{"snap", "channel"}, nil,
		),
		installBaseByCountryWeekly: prometheus.NewDesc("snapcraft_install_base_by_country_weekly",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. weekly_installed_base_by_country: similar to the installed_base_by_country metric but operates in a 7 day window.",
			[]string{"snap", "country"}, nil,
		),
		installBaseBySystemWeekly: prometheus.NewDesc("snapcraft_install_base_by_system_weekly",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. weekly_installed_base_by_operating_system: similar to the installed_base_by_operating_system metric but operates in a 7 day window.",
			[]string{"snap", "system"}, nil,
		),
		installBaseByVersionWeekly: prometheus.NewDesc("snapcraft_install_base_by_version_weekly",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. weekly_installed_base_by_version: similar to the installed_base_by_version metric but operates in a 7 day window.",
			[]string{"snap", "version"}, nil,
		),
	}
}

func (collector *SnapcraftCollector) Describe(ch chan<- *prometheus.Desc) {
	//Update this section with the each metric you create for a given collector
	ch <- collector.deviceChangeDaily
	ch <- collector.deviceChangeWeekly
	ch <- collector.installBaseByChannelDaily
	ch <- collector.installBaseByCountryDaily
	ch <- collector.installBaseBySystemDaily
	ch <- collector.installBaseByVersionDaily
	ch <- collector.installBaseByChannelWeekly
	ch <- collector.installBaseByCountryWeekly
	ch <- collector.installBaseBySystemWeekly
	ch <- collector.installBaseByVersionWeekly
}

func getSnapcraftMetrics(snapName string, snapMetricName string) SnapcraftMetrics {
	out, err := exec.Command("snapcraft", "metrics", snapName, "--name", snapMetricName, "--format", "json").Output()
	log.Printf("INFO snapcraft metrics %s --name %s --format json\n", snapName, snapMetricName)
	if err != nil {
		log.Printf("ERROR %v", err)
	}

	snapcraft := SnapcraftMetrics{}
	json.Unmarshal([]byte(out), &snapcraft)

	return snapcraft
}

func collectMetric(snapName string, snapMetricName string, metric *prometheus.Desc, ch chan<- prometheus.Metric) {
	snapcraftMetrics := getSnapcraftMetrics(snapName, snapMetricName)
	for _, item := range snapcraftMetrics.Series {
		for i, date := range snapcraftMetrics.Buckets {
			// metrics are on a 1 day lag, so date should match yesterday
			if time.Now().AddDate(0, 0, -1).Format("2006-01-02") == date {
				m := prometheus.MustNewConstMetric(metric, prometheus.GaugeValue, float64(item.Values[i]), snapName, item.Name)
				ch <- m
			}
		}
	}
}

func (collector *SnapcraftCollector) Collect(ch chan<- prometheus.Metric) {
	for _, snapName := range collector.SnapNames {
		log.Printf("INFO Started collecting metrics for %s \n", snapName)
		collectMetric(snapName, "daily_device_change", collector.deviceChangeDaily, ch)
		collectMetric(snapName, "weekly_device_change", collector.deviceChangeWeekly, ch)
		collectMetric(snapName, "installed_base_by_channel", collector.installBaseByChannelDaily, ch)
		collectMetric(snapName, "installed_base_by_country", collector.installBaseByCountryDaily, ch)
		collectMetric(snapName, "installed_base_by_operating_system", collector.installBaseBySystemDaily, ch)
		collectMetric(snapName, "installed_base_by_version", collector.installBaseByVersionDaily, ch)
		collectMetric(snapName, "weekly_installed_base_by_channel", collector.installBaseByChannelWeekly, ch)
		collectMetric(snapName, "weekly_installed_base_by_country", collector.installBaseByCountryWeekly, ch)
		collectMetric(snapName, "weekly_installed_base_by_operating_system", collector.installBaseBySystemWeekly, ch)
		collectMetric(snapName, "weekly_installed_base_by_version", collector.installBaseByVersionWeekly, ch)
		log.Printf("INFO Finished collecting metrics for %s \n", snapName)
	}
}

func main() {
	listenPort := os.Getenv("LISTEN_PORT")
	if listenPort == "" {
		listenPort = "9888"
	}
	metricsPath := os.Getenv("METRICS_PATH")
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	snapNamesString := os.Getenv("SNAP_NAMES")
	if snapNamesString != "" {
		snapNames := strings.Split(snapNamesString, ", ")
		log.Fatal(snapcraftExporter(listenPort, metricsPath, snapNames))
	}
}

func snapcraftExporter(listenPort string, metricsPath string, snapNames []string) error {
	snapcraft := newSnapcraftCollector(snapNames)
	prometheus.MustRegister(snapcraft)

	http.Handle(metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
            <html>
            <head><title>Snapcraft Metrics Exporter</title></head>
            <body>
            <h1>Snapcraft Exporter</h1>
            <p><a href='` + metricsPath + `'>Metrics</a></p>
            </body>
            </html>
        `))
	})

	return http.ListenAndServe(fmt.Sprintf(":%s", listenPort), nil)
}
