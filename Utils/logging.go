package Utils

import (
	"io"
	"os"
	"time"
)

// SetupLoggerWriter creates a new log file in the specified path named name. It then creates an io.Writer that writes to the file.
// If console is set to true, it will also write to os.Stdout.
func SetupLoggerWriter(path string, name string, console bool) io.Writer {
	if path == "" {
		panic("Path cannot be empty")
	}
	// check if goexpose directory exists in path
	var stat os.FileInfo
	var err error
	if stat, err = os.Stat(path); os.IsNotExist(err) {
		err = os.Mkdir(path, 0755)
		if err != nil {
			panic("Failed to create " + path + ":" + err.Error())
		}
	}
	// check if we can create a file in /var/logger/goexpose
	if stat != nil && !stat.IsDir() {
		panic(path + " is not a directory")
	}
	// create logger file
	file, err := os.OpenFile(path+"/"+name+time.Now().Format(time.RFC3339)+".log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	var writers []io.Writer
	writers = append(writers, file)
	if console {
		writers = append(writers, os.Stdout)
	}

	return io.MultiWriter(writers...)
}
