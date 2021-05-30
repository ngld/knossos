package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ngld/knossos/packages/libknossos/pkg/api"
)

var logLevelMap = map[api.LogLevel]zerolog.Level{
	api.LogDebug: zerolog.DebugLevel,
	api.LogInfo:  zerolog.InfoLevel,
	api.LogWarn:  zerolog.WarnLevel,
	api.LogError: zerolog.ErrorLevel,
	api.LogFatal: zerolog.FatalLevel,
}

func getConsoleWriter(out io.Writer) zerolog.ConsoleWriter {
	writer := zerolog.ConsoleWriter{Out: out}
	writer.TimeFormat = "02.01.2006 15:04:05 MST"
	writer.PartsOrder = []string{
		zerolog.TimestampFieldName,
		"req",
		zerolog.LevelFieldName,
		zerolog.CallerFieldName,
		zerolog.MessageFieldName,
	}

	// writer.PartsExclude = []string{"req"}
	writer.FormatFieldValue = func(value interface{}) string {
		if value == nil {
			return "           "
		}

		str, ok := value.(string)
		if ok {
			if len(str) == 11 {
				// color request IDs in cyan  we have to guess based on the field content because we can't get
				// the current field name
				return fmt.Sprintf("\x1b[%dm%s\x1b[0m", 36, value)
			} else if strings.Contains(str, "\\n") && strings.Contains(str, "\\t") {
				// unquote values that contain line breaks and tabs because they're most likely stack traces
				str, err := strconv.Unquote(str)
				if err == nil {
					return str
				}
			}
		}

		return fmt.Sprintf("%s", value)
	}

	writer.FormatCaller = func(caller interface{}) string {
		callerStr, ok := caller.(string)
		if !ok {
			return ""
		}

		parts := strings.SplitN(callerStr, ":", 3)
		if len(parts) == 1 {
			return parts[0]
		}

		if len(parts) == 3 {
			parts[0] = parts[0] + ":" + parts[1]
			parts[1] = parts[2]
		}

		wd, err := os.Getwd()
		if err != nil {
			fmt.Print(err)
			return callerStr
		}

		rel, err := filepath.Rel(wd, parts[0])
		if err != nil {
			fmt.Print(err)
			return callerStr
		}

		return fmt.Sprintf("\x1b[%dm%s:%s\x1b[0m \x1b[36m>\x1b[0m", 1, filepath.ToSlash(rel), parts[1])
	}

	return writer
}

func logCallback(level api.LogLevel, msg string, args ...interface{}) {
	log.WithLevel(logLevelMap[level]).CallerSkipFrame(3).Msgf(msg, args...)
}
