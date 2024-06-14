package ninjacrawler

import (
	"fmt"
	"github.com/spf13/viper"
)

// Config represents the application configuration.
type configService struct {
	v *viper.Viper // Viper instance for configuration management
}

// newConfig creates a new instance of Config.
func newConfig() *configService {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/")
	v.AllowEmptyEnv(true)
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		fmt.Printf("Error reading Config file: %v\n", err)
	}

	return &configService{v: v}
}

// Env retrieves a configuration value from environment variables.
func (c *configService) Env(envName string, defaultValue ...interface{}) interface{} {
	value := c.v.Get(envName)
	if value != nil {
		return value
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return nil
}

func (c *configService) EnvString(envName string, defaultValue ...string) string {
	value := c.v.Get(envName)
	if value != nil {
		return fmt.Sprint(value)
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return ""
}

// Add adds a configuration to the application.
func (c *configService) Add(name string, configuration interface{}) {
	c.v.Set(name, configuration)
}

// Get retrieves a configuration value from the application.
func (c *configService) Get(path string, defaultValue ...interface{}) interface{} {
	return c.v.Get(path)
}

// GetString retrieves a string type configuration value from the application.
func (c *configService) GetString(path string, defaultValue ...interface{}) string {
	return c.v.GetString(path)
}

// GetInt retrieves an integer type configuration value from the application.
func (c *configService) GetInt(path string, defaultValue ...interface{}) int {
	return c.v.GetInt(path)
}

// GetBool retrieves a boolean type configuration value from the application.
func (c *configService) GetBool(path string, defaultValue ...interface{}) bool {
	return c.v.GetBool(path)
}
