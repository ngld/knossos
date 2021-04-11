package buildsys

import (
	"encoding/gob"
	"os"
)

func init() {
	gob.Register(TaskList{})
	gob.Register(Task{})
	gob.Register(TaskCmdScript{})
	gob.Register(TaskCmdTaskRef{})
}

func WriteCache(file string, options map[string]string, list TaskList, scriptFiles []string) error {
	handle, err := os.Create(file)
	if err != nil {
		return err
	}
	defer handle.Close()

	encoder := gob.NewEncoder(handle)
	err = encoder.Encode(options)
	if err != nil {
		return err
	}

	err = encoder.Encode(list)
	if err != nil {
		return err
	}

	return encoder.Encode(scriptFiles)
}

func ReadCache(file string) (map[string]string, TaskList, []string, error) {
	handle, err := os.Open(file)
	if err != nil {
		return nil, nil, nil, err
	}
	defer handle.Close()

	decoder := gob.NewDecoder(handle)

	var options map[string]string
	err = decoder.Decode(&options)
	if err != nil {
		return nil, nil, nil, err
	}

	var result TaskList
	err = decoder.Decode(&result)
	if err != nil {
		return options, nil, nil, err
	}

	var scriptFiles []string
	err = decoder.Decode(&scriptFiles)
	if err != nil {
		return options, nil, nil, err
	}

	return options, result, scriptFiles, nil
}
