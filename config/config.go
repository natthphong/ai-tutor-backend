package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Env              string
	EnableS3         bool
	Server           Server
	LogConfig        LogConfig
	DBConfig         DBConfig
	HTTP             HTTP
	PermissionConfig map[string][]string
}

//type PermissionConfig struct {
//	Permissions map[string][]string
//}

type RedisConfig struct {
	Mode            string
	Host            string
	Port            string
	Password        string
	DB              int
	PoolTimeout     time.Duration
	DialTimeout     time.Duration
	WriteTimeout    time.Duration
	ReadTimeout     time.Duration
	ConnMaxIdleTime time.Duration
	Cluster         struct {
		Password string
		Addr     []string
	}
}

type AwsS3Config struct {
	DoSpaceEndpoint string
	DoSpaceRegion   string
	AccessKey       string
	SecretKey       string
	BucketName      string
}

type Server struct {
	Name string
	Port string
}

type LogConfig struct {
	Level string
}

type DBConfig struct {
	Host            string
	Port            string
	Username        string
	Password        string
	Name            string
	MaxOpenConn     int32
	MaxConnLifeTime int64
}

type HTTP struct {
	TimeOut            time.Duration
	MaxIdleConn        int
	MaxIdleConnPerHost int
	MaxConnPerHost     int
}

func InitConfig() (*Config, error) {

	viper.SetDefault("LogConfig.LEVEL", "info")

	configPath, ok := os.LookupEnv("API_CONFIG_PATH")
	if !ok {
		configPath = "./config"
	}

	configName, ok := os.LookupEnv("API_CONFIG_NAME")
	if !ok {
		configName = "config"
	}

	viper.SetConfigName(configName)
	viper.AddConfigPath(configPath)

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("config file not found. using default/env config: " + err.Error())
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var c Config

	err := viper.Unmarshal(&c)
	if err != nil {
		return nil, err
	}

	return &c, nil

}

func InitTimeZone() {
	ict, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		panic(err)
	}
	time.Local = ict
}
