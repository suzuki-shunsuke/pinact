package log

import (
	"github.com/sirupsen/logrus"
)

func New(version string) *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"pinact_version": version,
		"program":        "pinact",
	})
}

func SetLevel(level string, logE *logrus.Entry) {
	if level == "" {
		return
	}
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		logE.WithField("log_level", level).WithError(err).Error("the log level is invalid")
		return
	}
	logrus.SetLevel(lvl)
}

func SetColor(color string, logE *logrus.Entry) {
	switch color {
	case "", "auto":
		return
	case "always":
		logrus.SetFormatter(&logrus.TextFormatter{
			ForceColors: true,
		})
	case "never":
		logrus.SetFormatter(&logrus.TextFormatter{
			DisableColors: true,
		})
	default:
		logE.WithField("log_color", color).Error("log_color is invalid")
		return
	}
}
