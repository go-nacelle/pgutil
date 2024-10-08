package pgutil

import (
	"github.com/go-nacelle/nacelle/v2"
	_ "github.com/lib/pq"
)

type Initializer struct {
	Logger   nacelle.Logger           `service:"logger"`
	Services nacelle.ServiceContainer `service:"services"`
}

const ServiceName = "db"

func NewInitializer(configs ...ConfigFunc) *Initializer {
	// For expansion
	options := getOptions(configs)
	_ = options

	return &Initializer{}
}

func (i *Initializer) Init(config nacelle.Config) error {
	dbConfig := &Config{}
	if err := config.Load(dbConfig); err != nil {
		return err
	}

	logger := i.Logger
	if !dbConfig.LogSQLQueries {
		logger = nacelle.NewNilLogger()
	}

	db, err := Dial(dbConfig.DatabaseURL, logger)
	if err != nil {
		return err
	}

	return i.Services.Set(ServiceName, db)
}
