package buildsys

import (
	"encoding/gob"
	"os"

	"github.com/rotisserie/eris"
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
		return eris.Wrapf(err, "failed to create %s", file)
	}
	defer handle.Close()

	encoder := gob.NewEncoder(handle)
	err = encoder.Encode(options)
	if err != nil {
		return eris.Wrap(err, "failed to write options")
	}

	err = encoder.Encode(list)
	if err != nil {
		return eris.Wrap(err, "failed to write tasks")
	}

	err = encoder.Encode(scriptFiles)
	if err != nil {
		return eris.Wrap(err, "failed to write scripts")
	}

	return nil
}

func ReadCache(file string) (map[string]string, TaskList, []string, error) {
	handle, err := os.Open(file)
	if err != nil {
		return nil, nil, nil, eris.Wrapf(err, "failed to open %s", file)
	}
	defer handle.Close()

	decoder := gob.NewDecoder(handle)

	var options map[string]string
	err = decoder.Decode(&options)
	if err != nil {
		return nil, nil, nil, eris.Wrap(err, "failed to parse options")
	}

	var result TaskList
	err = decoder.Decode(&result)
	if err != nil {
		return options, nil, nil, eris.Wrap(err, "failed to parse tasks")
	}

	var scriptFiles []string
	err = decoder.Decode(&scriptFiles)
	if err != nil {
		return options, nil, nil, eris.Wrap(err, "failed to parse scripts")
	}

	return options, result, scriptFiles, nil
}
