package main

import (
	"context"
	"fmt"
	"os"

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
)

const (
	defaultPostgresHostname = "localhost"
	defaultPostgresPort     = 5432
	defaultPostgreSSLMode   = "disable"

	defaultPrometheusHostname = "localhost"
	defaultPrometheusPort     = "9090"
	defaultPrometheusQuery    = "namespace:container_cpu_usage_seconds_total:sum_rate"

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
	defer r.Queryer.Close(context.Background())

	err = r.Queryer.Ping(context.Background())
	if err != nil {
		return err
	}

	err = r.CreateDatabase(defaultDatabaseName)
	if err != nil {
		return fmt.Errorf("Failed to create the metering database: %+v", err)
	}
	// TODO: need to pass in a configuration file for creating the tables we're
	// interested in going forward.
	err = r.CreateTable(defaultTableName, defaultCheckIfTableExists)
	if err != nil {
		return fmt.Errorf("Failed to create the test table in the metering database: %+v", err)
	}

	// TODO: use something more robust to build up the Prometheus URL besides fmt.Sprintf
	apiClient, err := prom.NewPrometheusAPIClient(fmt.Sprintf("http://%s:%s", defaultPrometheusHostname, defaultPrometheusPort))
	if err != nil {
		return err
	}
	metrics, err := prom.ExecPromQuery(apiClient, defaultPrometheusQuery)
	if err != nil {
		return err
	}
	for _, metric := range metrics {
		err = r.InsertValuesIntoTable(defaultTableName, *metric)
		if err != nil {
			return err
		}
	}

	return nil
}
