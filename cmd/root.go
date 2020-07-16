package cmd

import (
	"fmt"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

const flagLogLevel = "loglevel"

var rootCmd = &cobra.Command{
	Use:   "debendabot",
	Short: "Dependabot for Debian",
	Long:  `debendabot - what if debian image updates could be locked and managed by Dependabot`,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		lvl, err := cmd.Flags().GetString(flagLogLevel)
		if err != nil {
			return err
		}
		parsed, err := logrus.ParseLevel(lvl)
		if err != nil {
			return err
		}
		logrus.SetLevel(parsed)

		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "15:04:05.000",
			FullTimestamp:   true,

		})
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.debendabot.yaml)")
	rootCmd.PersistentFlags().String(flagLogLevel, "info", "Log level")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".cli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".debendabot")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
