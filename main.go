package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const (
	Version = "1.0.0"
)

func main() {
	var showVersion bool
	var before string
	setting := Setting{}

	rootCmd := &cobra.Command{
		Use:   "s3-reinvoke-lambda [bucket] [lambda-arn]",
		Short: "S3 Objects Lambda Reinvoking Tool",
		Long:  "A tool to re-invoke a lambda function for all objects in an S3 bucket",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// Show version mode
			if showVersion {
				fmt.Println(Version)
				os.Exit(0)
			}

			// Context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigs
				cancel()
			}()

			// Setting
			setting.Bucket = args[0]
			setting.FunctionName = args[1]
			if before != "" {
				t, err := time.Parse(time.RFC3339, before)
				if err != nil {
					slog.Error("Invalid date format as RFC3339 (e.g. 2024-06-21T19:54:00+09:00)", "value", before, "error", err)
					os.Exit(1)
				}
				setting.ModifiedBefore = &t
			}

			// Dependencies
			awsClient, err := NewAwsClient()
			if err != nil {
				slog.Error("Failed to create AWS client", "error", err)
				os.Exit(1)
			}

			// Run application
			summary, err := Run(ctx, setting, awsClient)
			if err != nil {
				slog.Error("An error occurred", "error", err)
				os.Exit(1)
			} else {
				slog.Info("The lambda function have been re-invoked for objects in S3 bucket", "total", summary.Total, "completed", summary.Done, "errors", summary.Errored)
			}
		},
	}

	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version")
	rootCmd.Flags().IntVarP(&setting.Parallelism, "parallel", "P", 100, "Number of parallel invocations")
	rootCmd.Flags().StringVarP(&setting.Prefix, "prefix", "p", "", "Key prefix to filter objects")
	rootCmd.Flags().StringVarP(&setting.StartAfter, "start-after", "a", "", "Start after this key to filter objects")
	rootCmd.Flags().StringVarP(&before, "modified-before", "b", "", "Modified before this date to filter objects")
	rootCmd.Flags().StringSliceVarP(&setting.LowerExtensions, "ext", "x", nil, "Lowercased extensions to filter objects (e.g. '.jpg', '.png')")
	rootCmd.Flags().BoolVarP(&setting.DryRun, "dry-run", "d", false, "Dry run mode (no lambda invocation)")

	_ = rootCmd.Execute()
}
