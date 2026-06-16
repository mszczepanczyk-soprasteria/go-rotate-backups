package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	vembed "github.com/NoUseFreak/go-vembed"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stenic/go-rotate-backups/internal/drivers"
	"github.com/stenic/go-rotate-backups/internal/utils"
)

var (
	now time.Time = time.Now()

	// Config
	keepDaily   int
	keepWeekly  int
	keepMonthly int
	keepYearly  int
	targetDir   string
	driverName  string
	// Extras
	v         string
	dryRun    bool
	logFormat string
	// Hidden
	backupDate string
)

const (
	DateFormat string = "2006-01-02_15-04-05"
)

func init() {
	rootCmd.Version = fmt.Sprintf(
		"%s, build %s",
		vembed.Version.GetGitSummary(),
		vembed.Version.GetGitCommit(),
	)

	// Config
	rootCmd.Flags().IntVar(&keepDaily, "daily", 7, "Amount of daily backups to keep")
	rootCmd.Flags().IntVar(&keepWeekly, "weekly", 4, "Amount of weekly backups to keep")
	rootCmd.Flags().IntVar(&keepMonthly, "monthly", 12, "Amount of monthly backups to keep")
	rootCmd.Flags().IntVar(&keepYearly, "yearly", 5, "Amount of yearly backups to keep")
	rootCmd.Flags().StringVar(&targetDir, "target", "./backups", "Base location where backup live")
	rootCmd.Flags().StringVar(&driverName, "driver", "local", "Driver selection (local, s3, azblob)")

	// Extras
	rootCmd.Flags().StringVarP(&v, "verbosity", "v", logrus.InfoLevel.String(), "Log level (debug, info, warn, error, fatal, panic")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Don't change any files")
	rootCmd.Flags().StringVar(&logFormat, "log-format", "text", "Log format (text, json)")

	// Hidden
	rootCmd.Flags().StringVar(&backupDate, "date", "", "Testing: Set time of the backup")
	rootCmd.Flags().MarkHidden("date")
}

func main() {
	rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:   "go-rotate-backups [flags] files...",
	Short: "go-rotate-backups",
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := setUpLogs(os.Stdout, v); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, files []string) error {
		date, _ := cmd.Flags().GetString("date")

		files = resolveFiles(files)
		storageDriver, err := drivers.GetDriver(driverName)
		if err != nil {
			return err
		}
		storageDriver.SetTargetPath(targetDir)
		if err := storageDriver.Init(); err != nil {
			return err
		}
		if dryRun {
			storageDriver = &drivers.DryRunDriver{
				Wrapped: storageDriver,
			}
		}

		util := utils.Utils{
			Driver:     storageDriver,
			DateFormat: DateFormat,
		}

		if nowInput, err := time.Parse(DateFormat, date); err == nil {
			now = nowInput
		}

		if err := addFunc(util, cmd, files); err != nil {
			return err
		}

		return rotateFunc(util, cmd, files)
	},
}

func resolveFiles(files []string) []string {
	resolvedFiles := []string{}
	for _, file := range files {
		if info, err := os.Stat(file); err == nil {
			if info.IsDir() {
				filepath.Walk(file, func(path string, info os.FileInfo, err error) error {
					if !info.IsDir() {
						resolvedFiles = append(resolvedFiles, path)
					}
					return nil
				})
			} else {
				resolvedFiles = append(resolvedFiles, file)
			}
		} else {
			logrus.Warn("File not found: ", file)
		}
	}

	return resolvedFiles
}

func setUpLogs(out io.Writer, level string) error {
	logrus.SetOutput(out)

	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)

	switch logFormat {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		customFormatter := new(logrus.TextFormatter)
		customFormatter.DisableTimestamp = true
		logrus.SetFormatter(customFormatter)
	default:
		logrus.Error("Unknown log format: ", logFormat)
	}

	return nil
}

func addFunc(util utils.Utils, cmd *cobra.Command, files []string) error {
	daily, weekly, monthly, yearly := util.GetPaths(util.Driver.GetTargetPath())

	target := path.Join(daily, now.Format(DateFormat))
	if now.Month() == time.January && now.Day() == 1 {
		target = path.Join(yearly, now.Format(DateFormat))
	} else if now.Day() == 1 {
		target = path.Join(monthly, now.Format(DateFormat))
	} else if now.Weekday() == time.Monday {
		target = path.Join(weekly, now.Format(DateFormat))
	}

	logrus.Infof("Backing up %d files to %s", len(files), target)
	return util.CopyFiles(files, target)
}

func rotateFunc(util utils.Utils, cmd *cobra.Command, files []string) error {
	daily, weekly, monthly, yearly := util.GetPaths(util.Driver.GetTargetPath())

	if err := util.CleanFolder(daily, now.AddDate(0, 0, -keepDaily)); err != nil {
		return err
	}
	if err := util.CleanFolder(weekly, now.AddDate(0, 0, -keepWeekly*7)); err != nil {
		return err
	}
	if err := util.CleanFolder(monthly, now.AddDate(0, -keepMonthly, 0)); err != nil {
		return err
	}
	if err := util.CleanFolder(yearly, now.AddDate(-keepYearly, 0, 0)); err != nil {
		return err
	}

	return nil
}
