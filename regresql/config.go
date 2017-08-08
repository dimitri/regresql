package regresql

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	// "github.com/spf13/viper"
	"github.com/theherk/viper" // fork with write support
)

type config struct {
	Root  string
	PgUri string
}

func (s *Suite) getRegressConfigFile() string {
	return filepath.Join(s.RegressDir, "regress.yaml")
}

func (s *Suite) createRegressDir() error {
	stat, err := os.Stat(s.RegressDir)
	if err != nil || !stat.IsDir() {
		// Only create regressdir when it doesn't exists already
		fmt.Printf("Creating directory '%s'\n", s.RegressDir)
		err := os.Mkdir(s.RegressDir, 0755)
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("Directory '%s' already exists\n", s.RegressDir)
	}
	return nil
}

func (s *Suite) setupConfig(pguri string) {
	v := viper.New()
	configFile := s.getRegressConfigFile()

	v.Set("Root", s.Root)
	v.Set("pguri", pguri)

	fmt.Printf("Creating configuration file '%s'\n", configFile)
	v.WriteConfigAs(configFile)
}

func (s *Suite) readConfig() (config, error) {
	var config config
	v := viper.New()
	v.SetConfigType("yaml")
	configFile := s.getRegressConfigFile()

	data, err := ioutil.ReadFile(configFile)

	if err != nil {
		return config, fmt.Errorf("Failed to read config '%s': ",
			configFile,
			err)
	}

	v.ReadConfig(bytes.NewBuffer(data))
	v.Unmarshal(&config)

	return config, nil
}
