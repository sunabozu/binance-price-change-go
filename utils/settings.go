package utils

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// EnvDocument struct
type EnvDocument struct {
	Env Env `json:"env"`
}

// Env struct
type Env struct {
	BinanceKey    string `json:"binance_key"`
	BinanceSecret string `json:"binance_secret"`
	PushedKey     string `json:"pushed_key"`
	PushedSecret  string `json:"pushed_secret"`
}

func LoadKeys(filePath string) (*Env, error) {
	envFile, err := os.Open(filePath)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	defer envFile.Close()

	// parse
	byteValue, _ := ioutil.ReadAll(envFile)
	var envDocument EnvDocument
	json.Unmarshal(byteValue, &envDocument)

	// log.Printf("%+v\n", envDocument)

	return &envDocument.Env, nil
}

// get the parent path where the configs are
func GetParentPath() (string, error) {
	execPath, err := os.Executable()

	if err != nil {
		return "", err
	}

	// if it's "go run", return simple ".."
	if execPath[:5] == "/var/" {
		return "..", nil
	}

	return filepath.Dir(filepath.Dir(execPath)), nil
}
