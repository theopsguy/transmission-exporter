package main

import (
	"log/slog"
	"net/http"
	"os"

	arg "github.com/alexflint/go-arg"
	"github.com/joho/godotenv"
	transmission "github.com/metalmatze/transmission-exporter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
)

var (
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
)

func init() {
	version.Version = Version
	version.Revision = Revision
	version.Branch = Branch
	version.BuildUser = BuildUser
	version.BuildDate = BuildDate
}

// Config gets its content from env and passes it on to different packages
type Config struct {
	TransmissionAddr     string `arg:"env:TRANSMISSION_ADDR"`
	TransmissionPassword string `arg:"env:TRANSMISSION_PASSWORD"`
	TransmissionUsername string `arg:"env:TRANSMISSION_USERNAME"`
	WebAddr              string `arg:"env:WEB_ADDR"`
	WebPath              string `arg:"env:WEB_PATH"`
	WebConfigFile        string `arg:"env:WEB_CONFIG_FILE"`
	LogFormat            string `arg:"env:LOG_FORMAT"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		slog.Info("no .env present")
	}

	c := Config{
		WebPath:          "/metrics",
		WebAddr:          ":19091",
		TransmissionAddr: "http://localhost:9091",
		LogFormat:        "text",
	}

	arg.MustParse(&c)

	log := setupLogger(c.LogFormat)
	slog.SetDefault(log)

	log.Info("starting transmission-exporter", "version", version.Info())

	var user *transmission.User
	if c.TransmissionUsername != "" && c.TransmissionPassword != "" {
		user = &transmission.User{
			Username: c.TransmissionUsername,
			Password: c.TransmissionPassword,
		}
	}

	client := transmission.New(c.TransmissionAddr, user)

	prometheus.MustRegister(NewTorrentCollector(client, log))
	prometheus.MustRegister(NewSessionCollector(client, log))
	prometheus.MustRegister(NewSessionStatsCollector(client, log))

	http.Handle(c.WebPath, promhttp.Handler())

	if c.WebPath != "/" {
		landingConfig := web.LandingConfig{
			Name:        "Transmission Exporter",
			Description: "Prometheus exporter for Transmission",
			Version:     version.Info(),
			Links: []web.LandingLinks{
				{
					Address: c.WebPath,
					Text:    "Metrics",
				},
			},
		}
		landingPage, err := web.NewLandingPage(landingConfig)
		if err != nil {
			log.Error("failed to create landing page", "error", err)
		}
		http.Handle("/", landingPage)
	}

	flags := web.FlagConfig{
		WebListenAddresses: &[]string{c.WebAddr},
		WebConfigFile:      &c.WebConfigFile,
	}

	server := &http.Server{}
	if err := web.ListenAndServe(server, &flags, log); err != nil {
		log.Error(err.Error())
	}
}

func boolToString(true bool) string {
	if true {
		return "1"
	}
	return "0"
}

func setupLogger(format string) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)

	}

	return slog.New(handler)
}
