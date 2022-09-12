package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var AllSnapcraftMetricNames = []string{"daily_device_change", "weekly_device_change", "installed_base_by_channel", "installed_base_by_country", "installed_base_by_operating_system", "installed_base_by_version", "weekly_installed_base_by_channel", "weekly_installed_base_by_country", "weekly_installed_base_by_operating_system", "weekly_installed_base_by_version"}

type SnapcraftMetricsItem struct {
	Name     string `json:"name"`
	Values   []int  `json:"values"`
	Released bool   `json:"currently_released,omitempty"`
}

type SnapcraftMetric struct {
	Buckets    []SnapcraftDate        `json:"buckets"`
	MetricName string                 `json:"metric_name"`
	Series     []SnapcraftMetricsItem `json:"series"`
	SnapID     string                 `json:"snap_id"`
	Status     string                 `json:"status"`
}

type SnapcraftMetrics struct {
	Metrics []SnapcraftMetric `json:"metrics"`
}

type SnapcraftMetricFilter struct {
	SnapID     string        `json:"snap_id"`
	MetricName string        `json:"metric_name"`
	Start      SnapcraftDate `json:"start,omitempty"`
	End        SnapcraftDate `json:"end,omitempty"`
}

type SnapcraftMetricFilters struct {
	Filters []SnapcraftMetricFilter `json:"filters"`
}

type SnapcraftDate time.Time

func (date *SnapcraftDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*date = SnapcraftDate(t)
	return nil
}

func (date SnapcraftDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(date))
}

type SnapcraftCollector struct {
	SnapIDs                    []string
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

func newSnapcraftCollector(snapIDs []string) *SnapcraftCollector {
	return &SnapcraftCollector{
		SnapIDs: snapIDs,
		deviceChangeDaily: prometheus.NewDesc("snapcraft_device_change_daily",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. daily_device_change: contains the 3 series representing the number of new, continued and lost devices with the given snap installed compared to the previous day.",
			[]string{"snap_id", "change"}, nil,
		),
		deviceChangeWeekly: prometheus.NewDesc("snapcraft_device_change_weekly",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. weekly_device_change: similar to the ‘daily_device_change’ metric but operates on a 7 day window. i.e. new contains the number of devices that were seen during the last 7 days but not in the previous 7 day and so on for continued and lost.",
			[]string{"snap_id", "change"}, nil,
		),
		installBaseByChannelDaily: prometheus.NewDesc("snapcraft_install_base_by_channel_daily",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. installed_base_by_channel: contains one series per channel representing the number of devices with the given snap installed, channels with no data across the entire interval are omitted.",
			[]string{"snap_id", "channel"}, nil,
		),
		installBaseByCountryDaily: prometheus.NewDesc("snapcraft_install_base_by_country_daily",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. installed_base_by_country: contains one series per country representing the number of devices with the given snap installed.",
			[]string{"snap_id", "country"}, nil,
		),
		installBaseBySystemDaily: prometheus.NewDesc("snapcraft_install_base_by_system_daily",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. installed_base_by_operating_system: contains one series per operating_system representing the number of devices with the given snap installed.",
			[]string{"snap_id", "system"}, nil,
		),
		installBaseByVersionDaily: prometheus.NewDesc("snapcraft_install_base_by_version_daily",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. installed_base_by_version: contains one series per version representing the number of devices with the given snap installed.",
			[]string{"snap_id", "version"}, nil,
		),
		installBaseByChannelWeekly: prometheus.NewDesc("snapcraft_install_base_by_channel_weekly",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. weekly_installed_base_by_channel: similar to the installed_base_by_channel metric but operates in a 7 day window.",
			[]string{"snap_id", "channel"}, nil,
		),
		installBaseByCountryWeekly: prometheus.NewDesc("snapcraft_install_base_by_country_weekly",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. weekly_installed_base_by_country: similar to the installed_base_by_country metric but operates in a 7 day window.",
			[]string{"snap_id", "country"}, nil,
		),
		installBaseBySystemWeekly: prometheus.NewDesc("snapcraft_install_base_by_system_weekly",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. weekly_installed_base_by_operating_system: similar to the installed_base_by_operating_system metric but operates in a 7 day window.",
			[]string{"snap_id", "system"}, nil,
		),
		installBaseByVersionWeekly: prometheus.NewDesc("snapcraft_install_base_by_version_weekly",
			"Exported from https://snapcraft.io/docs/snapcraft-metrics. weekly_installed_base_by_version: similar to the installed_base_by_version metric but operates in a 7 day window.",
			[]string{"snap_id", "version"}, nil,
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

func getSnapcraftMetrics(snapIDs []string, snapMetricNames []string) SnapcraftMetrics {
	macaroon := os.Getenv("SNAP_STORE_MACAROON")
	if macaroon == "" {
		panic("SNAP_STORE_MACAROON must be set")
	}
	client := http.Client{}
	snapcraft := SnapcraftMetrics{}
	postBody := SnapcraftMetricFilters{}
	for _, id := range snapIDs {
		for _, name := range snapMetricNames {
			filter := SnapcraftMetricFilter{
				SnapID:     id,
				MetricName: name,
			}
			postBody.Filters = append(postBody.Filters, filter)
		}
	}
	jsonPostBody, err := json.Marshal(postBody)
	req, err := http.NewRequest(http.MethodPost, "https://dashboard.snapcraft.io/dev/api/snaps/metrics", bytes.NewBuffer(jsonPostBody))
	req.Header.Add("Authorization", macaroon)
	fmt.Print(macaroon)
	if err != nil {
		panic(err)
	}
	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	if res.StatusCode != 200 {
		panic(string(body))
	}
	json.Unmarshal([]byte(body), &snapcraft)
	fmt.Print(string(body))

	return snapcraft
}

func (collector *SnapcraftCollector) snapcraftToPrometheus(metricName string) *prometheus.Desc {
	switch {
	case metricName == "daily_device_change":
		return collector.deviceChangeDaily
	case metricName == "weekly_device_change":
		return collector.deviceChangeWeekly
	case metricName == "installed_base_by_channel":
		return collector.installBaseByChannelDaily
	case metricName == "installed_base_by_country":
		return collector.installBaseByCountryDaily
	case metricName == "installed_base_by_operating_system":
		return collector.installBaseBySystemDaily
	case metricName == "installed_base_by_version":
		return collector.installBaseByVersionDaily
	case metricName == "weekly_installed_base_by_channel":
		return collector.installBaseByChannelWeekly
	case metricName == "weekly_installed_base_by_country":
		return collector.installBaseByCountryWeekly
	case metricName == "weekly_installed_base_by_operating_system":
		return collector.installBaseBySystemWeekly
	case metricName == "weekly_installed_base_by_version":
		return collector.installBaseByVersionWeekly
	default:
		panic("trying to export an unsupported metric")
	}
}

func (collector *SnapcraftCollector) Collect(ch chan<- prometheus.Metric) {
	snapcraftMetrics := getSnapcraftMetrics(collector.SnapIDs, AllSnapcraftMetricNames)
	for _, metric := range snapcraftMetrics.Metrics {
		for _, item := range metric.Series {
			for i, date := range metric.Buckets {
				m := prometheus.MustNewConstMetric(collector.snapcraftToPrometheus(metric.MetricName), prometheus.GaugeValue, float64(item.Values[i]), metric.SnapID, item.Name)
				m = prometheus.NewMetricWithTimestamp(time.Time(date), m)
				ch <- m
			}
		}
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
	snapIDString := os.Getenv("SNAP_IDS")
	if snapIDString != "" {
		snapIDs := strings.Split(snapIDString, ", ")
		log.Fatal(snapcraftExporter(listenPort, metricsPath, snapIDs))
	}
}

func snapcraftExporter(listenPort string, metricsPath string, snapIDs []string) error {
	snapcraft := newSnapcraftCollector(snapIDs)
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
