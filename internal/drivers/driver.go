package drivers

import "fmt"

type Driver interface {
	// Implementation
	// Init is called once before any other method, use it to setup the driver
	Init() error	
	// ListDirs should return a list of directories in the given path
	ListDirs(path string) ([]string, error)
	// Mkdir should create a directory in the given path
	Mkdir(path string) error
	// Delete should delete the given path
	Delete(src string) error
	// Copy should copy a local file from src to dst, and return the number of bytes copied
	Copy(src, dst string) (int64, error)

	// baseDriver
	SetTargetPath(path string)
	GetTargetPath() string
}

type BaseDriver struct {
	TargetPath string
}

func (d *BaseDriver) SetTargetPath(targetPath string) {
	d.TargetPath = targetPath
}
func (d *BaseDriver) GetTargetPath() string {
	return d.TargetPath
}

var driverRegistry = map[string]Driver{}

func GetDriver(name string) (Driver, error) {
	if val, ok := driverRegistry[name]; ok {
		return val, nil
	}

	return nil, fmt.Errorf("Driver %s could not be found", name)
}

func AddDriver(name string, driver Driver) {
	driverRegistry[name] = driver
}
