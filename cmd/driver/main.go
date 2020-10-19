package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/jackc/pgx/v4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	runner "github.com/timflannagan1/scratch/pkg/postgres"
	prom "github.com/timflannagan1/scratch/pkg/prometheus"
)

var (
	pgCfg    runner.PostgresqlConfig
	promCfg  prom.PrometheusImporterConfig
	logLevel string

	rootCmd = &cobra.Command{
		Use:   "help",
		Short: "Wrapper around the Postgresql go-client",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	execCmd = &cobra.Command{
		Use:   "exec",
		Short: "Execute the Postgresql runner package",
		RunE:  execRunner,
	}
	defaultPromtheusQueries = []string{
		"metering:node_allocatable_cpu_cores",
		"metering:node_allocatable_memory_bytes",
		"metering:node_capacity_cpu_cores",
		"metering:node_capacity_memory_bytes",
		"metering:persistentvolumeclaim_capacity_bytes",
		"metering:persistentvolumeclaim_phase",
		"metering:persistentvolumeclaim_request_bytes",
		"metering:persistentvolumeclaim_usage_bytes",
		"metering:pod_limit_cpu_cores",
		"metering:pod_limit_memory_bytes",
		"metering:pod_persistentvolumeclaim_request_info",
		"metering:pod_request_cpu_cores",
		"metering:pod_request_memory_bytes",
		"metering:pod_usage_cpu_cores",
		"metering:pod_usage_memory_bytes",
	}
)

const (
	defaultLogLevel = "INFO"

	defaultPostgresHostname = "localhost"
	defaultPostgresPort     = 5432
	defaultPostgreSSLMode   = "disable"

	// Note: in case you're running outside of a Pod, use the monitoring routes
	// instead of relying on the Service DNS resolution.
	defaultPrometheusURI = "https://thanos-querier.openshift-monitoring.svc:9091"

	defaultDatabaseName       = "metering"
	defaultTableName          = "test"
	defaultCheckIfTableExists = true
)

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", defaultLogLevel, "Sets the log level verbosity level")

	rootCmd.PersistentFlags().StringVar(&pgCfg.Hostname, "postgres-address", defaultPostgresHostname, "The hostname of the Postgresql database instance.")
	rootCmd.PersistentFlags().IntVar(&pgCfg.Port, "postgres-port", defaultPostgresPort, "The port of the Postgresql database instance.")
	rootCmd.PersistentFlags().StringVar(&pgCfg.SSLMode, "postgres-ssl-mode", defaultPostgreSSLMode, "The sslMode configuration for how to authenticate to the Postgresql database instance.")
	rootCmd.PersistentFlags().StringVar(&pgCfg.DatabaseName, "postgres-database-name", "metering", "The name of an existing database in Postgresql.")

	rootCmd.PersistentFlags().StringVar(&promCfg.Hostname, "prometheus-address", defaultPrometheusURI, "The hostname of the Prometheus cluster instance")
	rootCmd.PersistentFlags().StringVar(&promCfg.BearerToken, "prometheus-bearer-token", "", "The path to the bearer token file used to authenticate to the Prometheus instance")
	rootCmd.PersistentFlags().BoolVar(&promCfg.SkipTLSVerification, "prometheus-tls-insecure", true, "Allow insecure connections to Prometheus")
}

func main() {
	rootCmd.AddCommand(execCmd)

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute the runner package: %+v", err)
		os.Exit(1)
	}
}

// execRunner is responsible for executing the runner package
func execRunner(cmd *cobra.Command, args []string) error {
	logger, err := setupLogger(logLevel)
	if err != nil {
		return err
	}

	promCfg.Address, err = url.Parse(promCfg.Hostname)
	if err != nil {
		return fmt.Errorf("Failed to parse the Prometheus URL: %v", err)
	}
	apiClient, err := prom.NewPrometheusAPIClient(promCfg)
	if err != nil {
		return err
	}

	r, err := runner.NewPostgresqlRunner(pgCfg, logger)
	if err != nil {
		return fmt.Errorf("Failed to initialize the PostgresqlRunner type: %+v", err)
	}
	defer r.Queryer.Close()

	err = r.CreateDatabase(defaultDatabaseName)
	if err != nil {
		return fmt.Errorf("Failed to create the metering database: %+v", err)
	}
	err = populatePostgresTables(logger, apiClient, *r)
	if err != nil {
		return err
	}

	logger.Infof("Runner has finished")

	return nil
}

func populatePostgresTables(logger logrus.FieldLogger, apiClient v1.API, r runner.PostgresqlRunner) error {
	// TODO: this is such a poor implementation but should get the job done.
	// TODO: should benchmark this eventually.
	// TODO: should add a pprof debug profile.
	// TODO: should throw this in a Goroutine and use a shared connection pool.
	// TODO: need a way to track the last import timestamp so we're not potentially
	//		 importing duplicate metrics and save a decent amount of overhead.
	//
	// For each metric we're interested in tracking, ensure a Postgres table has
	// been created, and attempt to populate that table with the resultant matrix
	// values that gets returned from the query_range API.
	errCh := make(chan error)

	var wg sync.WaitGroup
	// iterate over the list of the promQL queries we're interested in
	// tracking and attempt to create (if it doesn't already exist) a
	// table in Postgres before inserting metric values.
	for _, query := range defaultPromtheusQueries {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()

			err := r.CreateTable(strings.Replace(query, ":", "_", -1), defaultCheckIfTableExists)
			if err != nil {
				errCh <- fmt.Errorf("Failed to create the test table in the metering database: %+v", err)
			}
		}(query)

		wg.Wait()
	}

	close(errCh)
	for e := range errCh {
		logger.Fatal(e.Error())
	}

	for _, query := range defaultPromtheusQueries {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()

			var errArr []string
			metrics, err := prom.ExecPromQuery(logger, apiClient, query)
			if err != nil {
				errArr = append(errArr, fmt.Sprintf("Failed to execute the %s query: %v", query, err))
			}

			tableName := strings.Replace(query, ":", "_", -1)
			logger.Debugf("Inserting values into the %s table for the %s query", tableName, query)

			b := pgx.Batch{}
			for _, metric := range metrics {
				err = r.BatchInsertValuesIntoTable(&b, tableName, *metric)
				if err != nil {
					errArr = append(errArr, fmt.Sprintf("Failed to construct batch insert: %v", err))
				}
			}

			// TODO: need to handle the case where the length of the batch queue
			// is greater than an upper bound of ~50 and flush the rest of those
			// metrics into a new batch.
			logger.Debugf("Pre-Batch Insert Length: %d", b.Len())
			br := r.Queryer.SendBatch(context.Background(), &b)
			ct, err := br.Exec()
			if err != nil {
				errArr = append(errArr, fmt.Sprintf("failed to execute batch results: %+v", err))
			}
			logger.Debugf("Batch insert summary: %+v", ct.RowsAffected())

			if len(errArr) != 0 {
				logger.Error(fmt.Errorf(strings.Join(errArr, "\n")))
			}
		}(query)

		wg.Wait()
	}

	return nil
}

func setupLoggerWithFields(logLevel string, fields logrus.Fields) (logrus.FieldLogger, error) {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "01-02-2006 15:04:05",
	})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return nil, err
	}
	logger := logrus.WithFields(fields)
	logger.Logger.SetLevel(level)

	return logger, nil
}

// setupLogger is a helper function for standing up a straight-forward
// logrus.FieldLogger instance.
func setupLogger(logLevel string) (logrus.FieldLogger, error) {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "01-02-2006 15:04:05",
	})
	logger := logrus.New()
	l, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("Invalid log level: %s", logLevel)
	}
	logger.SetLevel(l)

	return logger, nil
}
