package drivers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func init() {
	AddDriver("local", &LocalDriver{})
}

type LocalDriver struct {
	BaseDriver
}

func (d *LocalDriver) Init() error {
	return nil
}

func (d *LocalDriver) ListDirs(path string) ([]string, error) {
	res := []string{}
	files, err := os.ReadDir(path)
	if err != nil {
		return res, err
	}

	for _, file := range files {
		if file.IsDir() {
			res = append(res, file.Name())
		}
	}

	return res, nil
}

func (d *LocalDriver) Mkdir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0750); err != nil {
			return err
		}
	}

	return nil
}

func (d *LocalDriver) Delete(src string) error {
	return os.RemoveAll(src)
}

func (d *LocalDriver) Copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	os.MkdirAll(filepath.Dir(dst), 0750)

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}
