package config

import (
	"fmt"
	"log"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	HttpPort string `env:"HTTP_PORT" env-default:"8080"`

	PGUser     string `env:"POSTGRES_USER" env-required:"true"`
	PGPassword string `env:"POSTGRES_PASSWORD" env-required:"true"`
	PGHost     string `env:"POSTGRES_HOST" env-default:"127.0.0.1"`
	PGPort     string `env:"POSTGRES_PORT" env-default:"5432"`
	PGDB       string `env:"POSTGRES_DB" env-required:"true"`

	RedisHost     string `env:"REDIS_HOST" env-default:"127.0.0.1"`
	RedisPort     string `env:"REDIS_PORT" env-default:"6379"`
	RedisPassword string `env:"REDIS_PASSWORD" env-default:""`

	JWTSecret     string        `env:"JWT_SECRET" env-required:"true"`
	JWTAccessTTL  time.Duration `env:"JWT_ACCESS_TTL" env-default:"15m"`
	JWTRefreshTTL time.Duration `env:"JWT_REFRESH_TTL" env-default:"168h"`
}

func (c *Config) PostgresDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.PGUser,
		c.PGPassword,
		c.PGHost,
		c.PGPort,
		c.PGDB,
	)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

func MustLoad() *Config {
	var cfg Config
	if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
		if envErr := cleanenv.ReadEnv(&cfg); envErr != nil {
			log.Fatalf("cannot read config: %v (env err: %v)", err, envErr)
		}
	}

	return &cfg
}
