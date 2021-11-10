package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/ghodss/yaml"

	consul "github.com/hashicorp/consul/api"
	vault "github.com/hashicorp/vault/api"
	"go.uber.org/zap"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
)

const configDir = "./"
const (
	consulHttpToken = "CONSUL_HTTP_TOKEN"
	consulHttpAddr  = "CONSUL_HTTP_ADDR"
	pathApp         = "CONFIG_PATH_APP"
	configPathKey   = "CONFIG_PATH_KEY"
)

const (
	vaultAddr     = "VAULT_ADDR"
	vaultRoleId   = "VAULT_ROLE_ID"
	vaultSecretId = "VAULT_SECRET_ID"
)

type Options struct {
	Dir                string // dir where configs located, set to "./config/" if empty
	Type               string // yaml or json. Set to yaml if empty
	DevFile            string // dev config file name, set to dev.(yaml|json) if empty
	ProdFile           string // prod config file name, set to main.(yaml|json) if empty
	ReplaceFromEnvVars bool   // replace config values from ENV VARS, false by default
	EnvVarsPrefix      string // prefix for ENV VARS, empty by default
}

func (o *Options) fill() error {
	if o.Type == "" {
		o.Type = "yaml"
	}
	if o.Type != "json" && o.Type != "yaml" {
		return errors.Errorf("bad format %s, must be json or yaml", o.Type)
	}
	if o.DevFile == "" {
		o.DevFile = "dev." + o.Type
	}
	if o.ProdFile == "" {
		o.ProdFile = "main." + o.Type
	}
	if o.Dir == "" {
		o.Dir = configDir
	}
	if !strings.HasSuffix(o.Dir, string(os.PathSeparator)) {
		o.Dir = o.Dir + string(os.PathSeparator)
	}
	return nil
}
func Parse(configStruct interface{}, opts Options, log *zap.Logger) error {
	if log == nil {
		log = zap.NewNop()
	}

	t := reflect.TypeOf(configStruct)
	if t.Kind() != reflect.Ptr {
		return errors.New("configStruct arg must be pointer")
	}
	if t.Elem().Kind() != reflect.Struct {
		return errors.New("configStruct arg must be pointer to struct")
	}

	err := opts.fill()
	if err != nil {
		return err
	}
	loadedFromFile, err := loadFromFile(log, opts)
	if err != nil {
		return err
	}

	loadedFromConsul, err := loadFromConsul(log, opts)
	if err != nil {
		return err
	}

	loadedFromVault, err := loadFromVault(log)
	if err != nil {
		return err
	}

	if !loadedFromFile && !loadedFromConsul && !loadedFromVault {
		return errors.New("cannot load from config, please set at least one of sources: file, consul, vault")
	}

	if opts.ReplaceFromEnvVars {
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		viper.AllowEmptyEnv(true)
		viper.SetEnvPrefix(opts.EnvVarsPrefix)
		viper.AutomaticEnv()
	}

	err = viper.Unmarshal(configStruct)
	if err != nil {
		return err
	}

	// some checks
	validate := validator.New()
	err = validate.Struct(configStruct)
	if err != nil {
		return err
	}
	return nil

}

func loadFromFile(log *zap.Logger, opts Options) (loaded bool, err error) {
	configPath := opts.Dir + opts.DevFile
	if err := fileExists(configPath); err != nil {
		log.Debug("dev file not exists", zap.String("path", configPath), zap.Error(err))
		configPath = opts.Dir + opts.ProdFile
	}

	if err := fileExists(configPath); err != nil {
		log.Debug("prod file not exists", zap.String("path", configPath), zap.Error(err))
		return false, nil
	}

	log.Info("load config from file", zap.String("path", configPath))
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return false, err
	}
	viper.SetConfigFile(configPath)
	viper.SetConfigType(opts.Type)
	//fmt.Printf("start parsing config file %s, env prefix %s\n", configPath, prefix)
	err = viper.ReadInConfig()
	if err != nil {
		return false, err
	}
	log.Info("done loading config from file")
	return true, nil
}

func loadFromConsul(log *zap.Logger, opts Options) (loaded bool, err error) {
	if os.Getenv(consulHttpAddr) == "" || os.Getenv(pathApp) == "" || os.Getenv(configPathKey) == "" {
		log.Info("skip loading from consul, empty env variable(s)")
		return false, nil
	}
	log.Info("loading from consul", zap.String("addr", os.Getenv(consulHttpAddr)),
		zap.String("path", os.Getenv(pathApp)), zap.String("key", os.Getenv(configPathKey)))
	consulConfig := consul.DefaultConfig()
	consulConfig.WaitTime = 10 * time.Second
	client, err := consul.NewClient(consulConfig)
	if err != nil {
		return false, err
	}
	kv := client.KV()
	kvp, _, err := kv.Get(os.Getenv(pathApp)+"/"+os.Getenv(configPathKey), nil)
	if err != nil {
		return false, err
	}

	if kvp == nil {
		return false, errors.Errorf("no data at path %s key %s", os.Getenv(pathApp), os.Getenv(configPathKey))
	}
	viperConfig := make(map[string]interface{})

	switch opts.Type {
	case "json":
		if err = json.Unmarshal(kvp.Value, &viperConfig); err != nil {
			return false, err
		}
	case "yaml":
		if err = yaml.Unmarshal(kvp.Value, &viperConfig); err != nil {
			return false, err
		}
	}
	if err = viper.MergeConfigMap(viperConfig); err != nil {
		return false, err
	}
	log.Info("done loading from consul")

	return true, nil
}
func loadFromVault(log *zap.Logger) (loaded bool, err error) {
	if os.Getenv(vaultAddr) == "" {
		log.Info("skip loading from vault")
		return false, nil
	}
	log.Info("loading from vault", zap.String("addr", os.Getenv(vaultAddr)),
		zap.String("path", os.Getenv(pathApp)), zap.String("key", os.Getenv(configPathKey)))
	vaultConfig := vault.DefaultConfig()
	vaultConfig.Timeout = 10 * time.Second
	vaultClient, err := vault.NewClient(vaultConfig)
	if err != nil {
		return false, err
	}

	token, err := authAppRole(vaultClient, os.Getenv(vaultRoleId), os.Getenv(vaultSecretId))
	if err != nil {
		return false, err
	}
	vaultClient.SetToken(token)
	data, err := vaultClient.Logical().Read(os.Getenv(pathApp) + "/data/" + os.Getenv(configPathKey))
	if err != nil {
		return false, err
	}
	if data == nil {
		log.Warn("path not exists", zap.String("path", os.Getenv(pathApp)+"/data/"+os.Getenv(configPathKey)))
		return false, nil
	}
	err = viper.MergeConfigMap(data.Data["data"].(map[string]interface{}))
	if err != nil {
		return false, err
	}

	return true, nil

}
func authAppRole(client *vault.Client, roleId, secretId string) (string, error) {
	requestPath := "auth/approle/login"
	options := map[string]interface{}{
		"role_id":   roleId,
		"secret_id": secretId,
	}
	secret, err := client.Logical().Write(requestPath, options)
	if err != nil {
		return "", err
	}
	return secret.Auth.ClientToken, nil
}

func fileExists(path string) (err error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("file not exists: " + absPath)
		}
		return
	}
	if info.IsDir() {
		return errors.New("must be file: " + absPath)
	}
	return
}
