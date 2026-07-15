package config_test

import (
	"os"

	"github.com/renevo/config"
)

func ExampleSet_Bind() {
	settings := config.NewSet()

	// create just a simple struct
	myConfig := struct {
		Name     string `description:"This is a name"`
		Password string `description:"Super secret password" mask:"true"`
		HTTP     struct {
			Addr string `name:"Address" description:"Address to listen"`
			Port int16  `description:"What port to listen"`
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
	bindErr := applicationSet.Bind(&myConfig)
	if bindErr != nil {
		panic(bindErr)
	}

	// manually update a setting by full path (the value being set can come from os.GetEnv())
	_, _ = settings.Update("MyApplication.Enabled", "true")

	// dump the output
	_ = settings.Dump(os.Stdout)

	// Output:
	// Path                        Type        Value              Default Value      Description
	// Myapplication.Enabled       *bool       "true"             "false"            Enable something
	// Myapplication.Http.Addr     *string     "0.0.0.0"          "0.0.0.0"          Address to listen
	// Myapplication.Http.Port     *int16      "8080"             "8080"             What port to listen
	// Myapplication.Name          *string     "Default User"     "Default User"     This is a name
	// Myapplication.Password      *string     "*****"            "*****"            Super secret password
}
