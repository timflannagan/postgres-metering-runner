package main

import (
	"fmt"
	"os"
	"strings"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	_ "github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	runner "github.com/timflannagan1/scratch/pkg/postgres"
	prom "github.com/timflannagan1/scratch/pkg/prometheus"
)

var (
	cfg      runner.PostgresqlConfig
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
	defaultPostgresHostname = "localhost"
	defaultPostgresPort     = 5432
	defaultPostgreSSLMode   = "disable"

	defaultPrometheusHostname = "localhost"
	defaultPrometheusPort     = "9090"

	defaultDatabaseName       = "metering"
	defaultTableName          = "test"
	defaultCheckIfTableExists = true
)

func init() {
	// TODO: need to add configuration flags for the Prometheus connection
	rootCmd.PersistentFlags().StringVar(&cfg.Hostname, "postgres-address", defaultPostgresHostname, "The hostname of the Postgresql database instance. Defaults to localhost.")
	rootCmd.PersistentFlags().IntVar(&cfg.Port, "postgres-port", defaultPostgresPort, "The hostname of the Postgresql database instance. Defaults to localhost.")
	rootCmd.PersistentFlags().StringVar(&cfg.SSLMode, "postgres-ssl-mode", defaultPostgreSSLMode, "The sslMode configuration for how to authenticate to the Postgresql database instance.")
	rootCmd.PersistentFlags().StringVar(&cfg.DatabaseName, "postgres-database-name", "metering", "The name of an existing database in Postgresql.")
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
	fmt.Println("Attempting to setup a connection to the Postgresql instance")

	// TODO: should be using a connection pool and not piggy backing off a single
	// thread as it's not thread safe.
	r, err := runner.NewPostgresqlRunner(cfg)
	if err != nil {
		return fmt.Errorf("Failed to initialize the PostgresqlRunner type: %+v", err)
	}
	defer r.Queryer.Close()

	err = r.CreateDatabase(defaultDatabaseName)
	if err != nil {
		return fmt.Errorf("Failed to create the metering database: %+v", err)
	}

	// TODO: use something more robust to build up the Prometheus URL besides fmt.Sprintf
	apiClient, err := prom.NewPrometheusAPIClient(fmt.Sprintf("http://%s:%s", defaultPrometheusHostname, defaultPrometheusPort))
	if err != nil {
		return err
	}
	err = populatePostgresTables(apiClient, *r)
	if err != nil {
		return err
	}

	fmt.Println("Runner has finished")

	return nil
}

func populatePostgresTables(apiClient v1.API, r runner.PostgresqlRunner) error {
	// TODO: this is such a poor implementation but should get the job done.
	// TODO: should benchmark this eventually.
	// TODO: should add a pprof debug profile.
	// TODO: should throw this in a Goroutine and use a shared connection pool.
	// TODO: need a way to track the last import timestamp so we're not potentially
	//		 importing duplicate metrics and save a decent amount of overhead.
	// TODO: should probably create the table first, then insert into it using
	// the same connection tag that conn.Exec returns.
	//
	// For each metric we're interested in tracking, ensure a Postgres table has
	// been created, and attempt to populate that table with the resultant matrix
	// values that gets returned from the query_range API.
	errChan := make(chan error, 2)
	for _, query := range defaultPromtheusQueries {
		go func(query string) {
			err := r.CreateTable(strings.Replace(query, ":", "_", -1), defaultCheckIfTableExists)
			if err != nil {
				errChan <- fmt.Errorf("Failed to create the test table in the metering database: %+v", err)
			}
		}(query)
	}

	for _, query := range defaultPromtheusQueries {
		metrics, err := prom.ExecPromQuery(apiClient, query)
		if err != nil {
			return err
		}
		tableName := strings.Replace(query, ":", "_", -1)

		fmt.Println("Inserting values into the", tableName, "table for the", query, "promQL query")
		for _, metric := range metrics {
			go func(metric *prom.PrometheusMetric) {
				err = r.InsertValuesIntoTable(tableName, *metric)
				if err != nil {
					errChan <- err
				}
			}(metric)
		}
	}

	select {
	case err := <-errChan:
		fmt.Println(err.Error())
	}

	return nil
}
