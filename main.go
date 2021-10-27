package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/graph-gophers/graphql-go"
	"github.com/jmoiron/sqlx"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"github.com/rislah/fakes/gql"
	app "github.com/rislah/fakes/internal"
	"github.com/rislah/fakes/internal/circuitbreaker"
	"github.com/rislah/fakes/internal/geoip"
	"github.com/rislah/fakes/internal/jwt"
	"github.com/rislah/fakes/internal/postgres"
	"github.com/rislah/fakes/loaders"

	"github.com/rislah/fakes/schema"

	"github.com/rislah/fakes/internal/redis"
	"github.com/rislah/fakes/resolvers"

	"github.com/cep21/circuit/v3"
	"github.com/rislah/fakes/internal/local"
	"github.com/rislah/fakes/internal/logger"
)

type config struct {
	ListenAddr  string `default:":8080"`
	Environment string `default:"development"`
	PgHost      string `default:"127.0.0.1"`
	PgPort      string `default:"5432"`
	PgUser      string `default:"user"`
	PgPass      string `default:"parool"`
	PgDB        string `default:"user"`
	RedisHost   string `default:"localhost"`
	RedisPort   string `default:"6379"`
}

func main() {
	var conf config
	if err := envconfig.Process("fakes", &conf); err != nil {
		log.Fatal(err)
	}

	log := logger.New(conf.Environment)
	// geoIPDB := initGeoIPDB("./GeoLite2-Country.mmdb")
	jwtWrapper := jwt.NewHS256Wrapper(app.JWTSecret)
	userDB, conn := initUserDB(conf, log)
	roleDB := initRoleDB(conf, log)
	authenticator := app.NewAuthenticator(userDB, roleDB, jwtWrapper)
	userBackend := app.NewUserBackend(userDB, jwtWrapper)

	data := &app.Data{
		UserDB:        userDB,
		DB:            conn,
		Authenticator: authenticator,
		User:          userBackend,
		RoleDB:        roleDB,
	}

	rootResolver := resolvers.NewRootResolver(data)
	schemaStr, err := schema.String()
	if err != nil {
		log.Fatal("schema", err)
	}
	schema := graphql.MustParseSchema(schemaStr, rootResolver)
	_ = schema

	dl := loaders.New(data, conn, userDB, userBackend)

	m := mux.NewRouter()
	m.Use(dl.AttachMiddleware)
	// m.Handle("/query", &relay.Handler{Schema: schema}).Methods("POST")
	m.Handle("/query", &gql.Handler{Schema: schema}).Methods("POST")
	http.ListenAndServe(":8080", m)
}

func initHTTPServer(addr string, handler http.Handler) *http.Server {
	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return httpSrv
}

func fn() error {
	e1 := errors.New("error")
	e2 := errors.Wrap(e1, "inner")
	e3 := errors.Wrap(e2, "middle")
	return errors.Wrap(e3, "outer")
}

func initUserDB(conf config, log *logger.Logger) (app.UserDB, *sqlx.DB) {
	switch conf.Environment {
	case "local":
		return local.NewUserDB(), nil
	case "development":
		opts := postgres.Options{
			ConnectionString: fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", conf.PgHost, conf.PgPort, conf.PgUser, conf.PgPass, conf.PgDB),
			MaxIdleConns:     100,
			MaxOpenConns:     100,
		}

		client, err := postgres.NewClient(opts)
		if err != nil {
			log.Fatal("init postgres client", err)
		}

		userDBCircuit, err := circuitbreaker.New("postgres_userdb", circuitbreaker.Config{})
		if err != nil {
			log.Fatal("error creating userdb circuit", err)
		}

		redisCircuit, err := circuitbreaker.New("redis_cache_userdb", circuitbreaker.Config{})
		if err != nil {
			log.Fatal("error creating redis cache circuit", err)
		}

		rd := initRedis(conf, redisCircuit, log)
		db, err := postgres.NewCachedUserDB(client, rd, userDBCircuit)
		if err != nil {
			log.Fatal("init cached userdb", err)
		}

		return db, client
	default:
		panic("unknown environment")
	}
}

func initRoleDB(conf config, log *logger.Logger) app.RoleDB {
	opts := postgres.Options{
		ConnectionString: fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", conf.PgHost, conf.PgPort, conf.PgUser, conf.PgPass, conf.PgDB),
		MaxIdleConns:     100,
		MaxOpenConns:     100,
	}
	client, err := postgres.NewClient(opts)
	if err != nil {
		log.Fatal("init postgres client", err)
	}

	roleDBCircuit, err := circuitbreaker.New("postgres_roledb", circuitbreaker.Config{})
	if err != nil {
		log.Fatal("roledb circuit", err)
	}

	db := postgres.NewRoleDB(client, roleDBCircuit)

	return db
}

// func initMetrics(cm *circuit.Manager) metrics.Metrics {
// 	statsEngine := stats.NewEngine("app", stats.DefaultEngine.Handler)
// 	statsEngine.Register(prom.DefaultHandler)

// 	mtr := metrics.New(statsEngine)
// 	scf := metrics.NewCommandFactory(mtr)
// 	sf := rolling.StatFactory{}

// 	cm.DefaultCircuitProperties = append(cm.DefaultCircuitProperties, scf.CommandProperties, sf.CreateConfig)

// 	return mtr
// }

func initRedis(conf config, cb *circuit.Circuit, log *logger.Logger) redis.Client {
	redis, err := redis.NewClient(fmt.Sprintf("%s:%s", conf.RedisHost, conf.RedisPort), cb, log)
	if err != nil {
		log.Fatal("init redis", err)
	}
	return redis
}

func initGeoIPDB(filePath string) geoip.GeoIP {
	geoIPDB, err := geoip.New(filePath)
	if err != nil {
		log.Fatal("opening geoip database", err)
	}
	return geoIPDB
}
