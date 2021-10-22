package integration_tests

import (
	"database/sql"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cep21/circuit"
	"github.com/golang-migrate/migrate/v4"
	migratePostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	app "github.com/rislah/fakes/internal"
	"github.com/rislah/fakes/internal/circuitbreaker"
	"github.com/rislah/fakes/internal/postgres"
)

func getRootDir() string {
	_, b, _, _ := runtime.Caller(0)
	d := path.Join(path.Dir(b))
	return filepath.Dir(d)
}

func createMigrationInstance(conn *sql.DB, database string) (*migrate.Migrate, error) {
	config := &migratePostgres.Config{}
	driver, err := migratePostgres.WithInstance(conn, config)
	if err != nil {
		return nil, err
	}

	rootDir := getRootDir()
	if rootDir == "" {
		return nil, errors.New("rootdir is empty")
	}

	instance, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s/migrations", rootDir), database, driver)
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func makeUserDB() (app.UserDB, func() error, error) {
	cm := circuitbreaker.NewDefault()
	cb := cm.MustCreateCircuit(
		"integration_test",
		circuit.Config{
			Execution: circuit.ExecutionConfig{
				Timeout: 500 * time.Millisecond,
			},
		},
	)

	opts := postgres.Options{ConnectionString: "postgres://user:parool@localhost:5432/user?sslmode=disable"}
	conn, err := postgres.NewClient(opts)
	if err != nil {
		return nil, nil, err
	}

	db, err := postgres.NewUserDB(conn, cb)
	if err != nil {
		return nil, nil, err
	}

	migrationInstance, err := createMigrationInstance(conn, "user")
	if err != nil {
		return nil, nil, err
	}

	teardown := func() error {
		err := migrationInstance.Down()
		if err != nil {
			return err
		}
		err = migrationInstance.Up()
		if err != nil {
			return err
		}
		return nil
	}

	if err := migrationInstance.Up(); err != nil && err != migrate.ErrNoChange {
		return nil, nil, err
	}

	return db, teardown, nil
}