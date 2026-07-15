package config_test

import (
	"flag"
	"os"

	"github.com/renevo/config"
)

func ExampleSet_Bind() {
	settings := config.NewSet()

	// create just a simple struct with some descriptive flags for the configuration
	myConfig := struct {
		Name     string `description:"This is a name" flag:"name"`
		Password string `description:"Super secret password" mask:"true"`
		HTTP     struct {
			Addr string `name:"Address" description:"Address to listen" flag:"address"`
			Port int16  `description:"What port to listen" flag:"port"`
		}
		Enabled bool `description:"Enable something"`
	}{
		Name: "Default User",
	}

	// set values like normal
	myConfig.HTTP.Addr = "0.0.0.0"
	myConfig.HTTP.Port = 8080

	// bind the configuration under MyApplication to the pointer of the config
	applicationSet := settings.Subset("MyApplication")
	if applicationSet == nil {
		panic("unable to create application configuration set")
	}
	bindErr := applicationSet.Bind(&myConfig, config.WithFlagSet(flag.CommandLine))
	if bindErr != nil {
		panic(bindErr)
	}

	// parsing the flags, would normally be replaced with os.Args[1:]
	_ = flag.CommandLine.Parse([]string{"-name=flagged", "-address=127.0.0.1", "-port=8090"})

	// manually update a setting by full path (the value being set can come from os.GetEnv())
	_, _ = settings.Update("MyApplication.Enabled", "true")

	// dump the output
	_ = settings.Dump(os.Stdout)

	// Output:
	// Path                        Type        Value           Default Value      Description
	// Myapplication.Enabled       *bool       "true"          "false"            Enable something
	// Myapplication.Http.Addr     *string     "127.0.0.1"     "0.0.0.0"          Address to listen
	// Myapplication.Http.Port     *int16      "8090"          "8080"             What port to listen
	// Myapplication.Name          *string     "flagged"       "Default User"     This is a name
	// Myapplication.Password      *string     "*****"         "*****"            Super secret password
}
