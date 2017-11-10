package main

import (
	"encoding/json"
	"io/ioutil"
)

func saveJSON(filename string, v interface{}) error {
	bytes, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, bytes, 0666)
	return err
}

func loadJSON(filename string, v interface{}) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, v)
}
