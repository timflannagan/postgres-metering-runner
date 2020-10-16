package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/sirupsen/logrus"
	prom "github.com/timflannagan1/scratch/pkg/prometheus"
)

// PostgresqlConfig specifies how to connect to a Postgresql database instance.
type PostgresqlConfig struct {
	Hostname       string
	Port           int
	SSLMode        string
	DatabaseName   string
	ConnectOptions []string
}

// PostgresqlRunner is reponsible for managing a Postgresql client instance.
type PostgresqlRunner struct {
	Config  *PostgresqlConfig
	Queryer *pgxpool.Pool
	Logger  logrus.FieldLogger
}

// NewPostgresqlRunner is the constructor for the NewPostgresqlRunner type
func NewPostgresqlRunner(config PostgresqlConfig, logger logrus.FieldLogger) (*PostgresqlRunner, error) {
	// TODO: add a list of options that gets unpacked during the fmt.Sprintf call
	// TODO: hardcode the username/password for now as we control the Postgres manifest
	// that gets created.
	connString := fmt.Sprintf("postgresql://testuser:testpass@%s:%d/%s?connect_timeout=10", config.Hostname, config.Port, config.DatabaseName)

	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("Failed to construct a pgxpool.Config based on the %s connection string: %v", connString, err)
	}
	logger.Debugf("Postgres Configuration: %+v\n", cfg.ConnString())

	conn, err := pgxpool.Connect(context.Background(), cfg.ConnString())
	if err != nil {
		return nil, fmt.Errorf("Failed to create a connection to the configured Postgres instance: %+v", err)
	}

	return &PostgresqlRunner{
		Config:  &config,
		Queryer: conn,
		Logger:  logger,
	}, nil
}

// CreateDatabase is responsible for creating the @name database in Postgres.
func (r *PostgresqlRunner) CreateDatabase(databaseName string) error {
	// Note: Postgres has no notion of `if not exists`, so always attempt to create
	// the database. Workaround was implemented in the following stackoverflow post:
	// https://stackoverflow.com/questions/18389124/simulate-create-database-if-not-exists-for-postgresql
	createSQL := fmt.Sprintf("SELECT 'CREATE DATABASE %s' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '%s')", databaseName, databaseName)
	_, err := r.Queryer.Exec(context.Background(), createSQL)
	if err != nil {
		return err
	}

	return nil
}

// CreateTable is reponsible for creating the @tableName table that will store
// Prometheus metric data.
func (r *PostgresqlRunner) CreateTable(tableName string, checkIfExists bool) error {
	var ifNotExistsStr string
	if checkIfExists {
		ifNotExistsStr = "IF NOT EXISTS"
	}

	createSQL := fmt.Sprintf(`CREATE TABLE %s %s(amount float8, timestamp timestamptz, timePrecision float8, labels jsonb)`, ifNotExistsStr, tableName)
	_, err := r.Queryer.Exec(context.Background(), createSQL)
	if err != nil {
		return err
	}
	logrus.Infof("Processing the %s table for creation", tableName)

	return nil
}

// BatchInsertValuesIntoTable is responsible for building up an `insert into values`
// transaction and queueing that transaction in the @b batch queue.
func (r *PostgresqlRunner) BatchInsertValuesIntoTable(b *pgx.Batch, tableName string, metric prom.PrometheusMetric) error {
	labels, err := convertLabelsMapToJSON(metric.Labels)
	if err != nil {
		return err
	}

	b.Queue(fmt.Sprintf("INSERT INTO %s VALUES($1, $2, $3, ($4)::jsonb)", tableName), metric.Amount, metric.Timestamp, float64(metric.StepSize), labels)

	return nil
}

// InsertValuesIntoTable is responsible for inserting Prometheus metric values stored
// in the @metric type, marshalling the map[string]string metric.Labels into JSON,
// and performing the `insert into ... values(...)` call to the @tableName table.
func (r *PostgresqlRunner) InsertValuesIntoTable(tableName string, metric prom.PrometheusMetric) error {
	labels, err := convertLabelsMapToJSON(metric.Labels)
	if err != nil {
		return err
	}

	_, err = r.Queryer.Exec(context.Background(), fmt.Sprintf("INSERT INTO %s VALUES($1, $2, $3, ($4)::jsonb)", tableName), metric.Amount, metric.Timestamp, float64(metric.StepSize), labels)
	if err != nil {
		return err
	}

	return nil
}

// convertLabelsMapToJSON is a helper function responsible for translating
// a map of Prometheus metric labels into a JSON string.
func convertLabelsMapToJSON(labels map[string]string) ([]byte, error) {
	l, err := json.Marshal(labels)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal metric labels into a JSON string: %v", err)
	}
	return l, nil
}
