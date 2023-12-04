package initComponent

import (
	"github.com/magiconair/properties"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path/filepath"
	"reflect"
)

type TypeProp int

type Prop interface {
	Validate()
}

const (
	YML TypeProp = iota
	PROPERTIES
)

var filePath = "resources/properties.yml"
var props = make([]interface{}, 0)
var typeProp = YML

func SetFilePath(typeP TypeProp, path string) {
	typeProp = typeP
	filePath = path
}

func AddProp(p interface{}) {
	if value := reflect.ValueOf(p); value.Kind() != reflect.Ptr || value.Elem().Kind() != reflect.Struct {
		log.Fatal("The parameter must be a pointer or struct")
	}

	props = append(props, p)
}

func RunLoadProperties() {
	log.Println("Started load properties from file: ", filePath)

	filename, err := filepath.Abs(filePath)
	if err != nil {
		log.Fatal("Error get absolute path: ", err)
	}

	var dataEnv string
	var propEnv *properties.Properties

	if typeProp == YML {
		dataFile, err := os.ReadFile(filename)
		if err != nil {
			log.Fatal("Error read file: ", err)
		}
		dataEnv = os.ExpandEnv(string(dataFile))
	} else if typeProp == PROPERTIES {
		propEnv = properties.MustLoadFile(filename, properties.UTF8)
	}

	for _, p := range props {
		if typeProp == YML {
			err = yaml.Unmarshal([]byte(dataEnv), p)
		} else if typeProp == PROPERTIES {
			err = propEnv.Decode(p)
		}
		//reflect.TypeOf((*MyInterface)(nil)) returns a pointer to the interface, and Elem() is used to get the type of the interface.
		if value := reflect.TypeOf(p); value.Implements(reflect.TypeOf((*Prop)(nil)).Elem()) {
			p.(Prop).Validate()
		}
		if err != nil {
			log.Fatal("Error unmarshal properties: ", err)
		}
	}

	log.Println("Finished load properties from file: ", filePath)
}
